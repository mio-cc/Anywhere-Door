package op

import (
	"context"
	"fmt"
	"time"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

type cacheKey struct{ graph, branch string }

// cached 是一个 (图, 分支) 在某个版本的状态（E.05.01 §5.3）。建好以后只读：要改先 cloneDoc。
type cached struct {
	version      int64
	doc          *gen.Graph   // 规范化后的文档
	g            *graph.Graph // doc 的只读图
	diags        []gen.Diag   // Opts{ForRun: true} 的全部诊断
	lastFull     int64        // 最近一份全文的版本
	lastCoalesce time.Time    // 上一次拖动合并的时间（E.05.04）
}

func (p *Pipeline) cacheGet(k cacheKey) *cached {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cache[k]
}

func (p *Pipeline) cachePut(k cacheKey, c *cached) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[k] = c
}

// headState 是 head 版本的状态：缓存对得上就用缓存，否则从存储重建并放进缓存。
func (p *Pipeline) headState(ctx context.Context, r Repo, k cacheKey, head int64) (*cached, error) {
	if c := p.cacheGet(k); c != nil && c.version == head {
		return c, nil
	}
	c, err := p.load(ctx, r, k.graph, k.branch, head)
	if err != nil {
		return nil, err
	}
	p.cachePut(k, c)
	return c, nil
}

// load：Base(v) → 规范化全文 → 按日志逐条重放（不补全、不生成 ID，E.05-R7）→ 只读图 → 全量校验。
func (p *Pipeline) load(ctx context.Context, r Repo, graphID, branch string, v int64) (*cached, error) {
	snap, err := r.Base(ctx, graphID, branch, v)
	if err != nil {
		return nil, repoErr(err, fmt.Sprintf("图 %s 分支 %s 版本 %d", graphID, branch, v))
	}
	if snap.Doc == nil {
		return nil, fmt.Errorf("op: 图 %s 版本 %d 没有全文", graphID, snap.Version)
	}
	doc := cloneDoc(snap.Doc)
	normalizeDoc(doc, p.o.View)
	m := newMutable(doc)
	e := p.env(true, true)
	for i := range snap.Ops {
		rec := snap.Ops[i]
		o := rec.Op
		if _, _, err := p.applyOne(m, &o, e); err != nil {
			return nil, fmt.Errorf("op: 重放图 %s 第 %d 版（%s）失败：%w", graphID, rec.Version, o.Kind, err)
		}
		m.doc.Version = rec.Version
	}
	m.doc.Version = v
	g, err := graph.FromDoc(m.doc)
	if err != nil {
		return nil, fmt.Errorf("op: 图 %s 版本 %d 加载失败：%w", graphID, v, err)
	}
	return &cached{version: v, doc: m.doc, g: g, diags: nonNil(schema.Validate(g, p.o.View, schema.Opts{ForRun: true})), lastFull: snap.Version}, nil
}

func nonNil(ds []gen.Diag) []gen.Diag {
	if ds == nil {
		return []gen.Diag{}
	}
	return ds
}

// Read 读某个版本的图和它的全量诊断（error 与 warn）；version 为 Head 时读 head（E.05.01 §5.3）。
func (p *Pipeline) Read(ctx context.Context, graphID, branch string, version int64) (*graph.Graph, []gen.Diag, error) {
	head, err := p.o.Repo.Head(ctx, graphID, branch)
	if err != nil {
		return nil, nil, repoErr(err, fmt.Sprintf("图 %s 分支 %s", graphID, branch))
	}
	if version > head || version < Head {
		return nil, nil, notFoundf("图 %s 分支 %s 没有版本 %d（head %d）", graphID, branch, version, head)
	}
	if version == Head || version == head {
		c, err := p.headState(ctx, p.o.Repo, cacheKey{graphID, branch}, head)
		if err != nil {
			return nil, nil, err
		}
		return c.g, append([]gen.Diag{}, c.diags...), nil
	}
	c, err := p.load(ctx, p.o.Repo, graphID, branch, version)
	if err != nil {
		return nil, nil, err
	}
	return c.g, c.diags, nil
}

// statsOf 是图的规模（E.05.01 §5.4）。
func statsOf(g *graph.Graph) gen.GraphMetaStats {
	var st gen.GraphMetaStats
	depth := 0
	for _, s := range g.Scopes() {
		st.Nodes += int64(len(s.Nodes()))
		if s.ID() != graph.RootScope {
			st.Subgraphs++
		}
		if d := g.Depth(s.ID()); d > depth {
			depth = d
		}
	}
	st.Layers = int64(depth + 1)
	return st
}
