package op

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mio-cc/anywhere-door/engine/authz"
	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/engine/schema/migrate"
	"github.com/mio-cc/anywhere-door/gen"
)

// Head 给 Read 用：读 head（0 是建图时的版本，P.09 K262 ⑥）。
const Head int64 = -1

// fullEvery：每隔多少个版本存一份全文（E.05.01 §5.1 第 8 步）。
const fullEvery = 50

// Options 是流水线的依赖（E.05.01 §4）。Repo、View、Authz、Now、NewID 必填。
type Options struct {
	Repo       Repo
	View       schema.View   // 注册表的实时视图（Registry.Live()）
	Authz      authz.Checker // M1 为 authz.Fixed{}（E.05.05）
	Estop      Estop         // 可为 nil：没有急停（M3）
	Proposer   Proposer      // 可为 nil：不能转为提议（M4）
	Bus        Bus           // 可为 nil
	Runs       RunNotifier   // 可为 nil
	Now        func() time.Time
	NewID      func() string
	Migrations []migrate.Step // nil 时用 migrate.Chain
	Logger     *slog.Logger
}

// Proposer 在同一个事务里存提议，返回提议 ID（E.17.03 实现，M4）。
type Proposer interface {
	Propose(ctx context.Context, r Repo, b Batch, res []authz.Result) (string, error)
}

// Bus 在事务提交之后收到这一批的事件；不能阻塞。
type Bus interface{ Publish(evs []gen.GraphEvent) }

// RunNotifier 收到图的新版本（E.16.02 实现）。
type RunNotifier interface {
	GraphChanged(graphID, branch string, version int64)
}

// Pipeline 是 Op 流水线：修改图的唯一入口（INV-08、E.05-R1）。
type Pipeline struct {
	o     Options
	mu    sync.Mutex // 保护 locks、cache
	locks map[string]*sync.Mutex
	cache map[cacheKey]*cached
}

// New 建流水线；必填项缺一个就 panic。
func New(o Options) *Pipeline {
	if o.Repo == nil || o.View == nil || o.Authz == nil || o.Now == nil || o.NewID == nil {
		panic("op.New：Repo、View、Authz、Now、NewID 都必填")
	}
	if o.Migrations == nil {
		o.Migrations = migrate.Chain
	}
	return &Pipeline{o: o, locks: map[string]*sync.Mutex{}, cache: map[cacheKey]*cached{}}
}

type mode int

const (
	modeNormal mode = iota
	modeUndo
	modeRedo
)

// Batch 是一批 Op（E.05.01 §4）。小写的字段只由本包设（撤销、重做、撤回、格式升级）。
type Batch struct {
	GraphID, Branch string
	BaseVersion     int64
	GroupID         string    // 空：整批共用一个（Op 里填的或新生成的）
	Actor           gen.Actor // 调用方按鉴权结果填
	Approver        *gen.Actor
	MandateID       string
	Taint           bool
	RunID           string
	Ops             []gen.Op

	internal  bool // 服务器生成的 Op：不再解码校验，可以有 restore_nodes
	mode      mode
	migrating bool
	pre       hook                                                 // 事务里、读到 head 之后先调用，生成 Ops（E.05.02）
	onTx      func(ctx context.Context, r Repo, res *Result) error // 落库之后、同一个事务里调用（E.05.02）
}

type hook func(ctx context.Context, p *Pipeline, r Repo, c *cached, b *Batch) error

// Result 是提交的结果（HTTP 200 的 OpsApplied，或 202 的提议）。
type Result struct {
	Version  int64
	Applied  []string // 落库的 op id，按版本顺序
	Rebased  bool
	IDMap    map[string]string // M1 一直是空对象（临时 ID 在 M4）
	Diags    []gen.Diag        // 提交后整张图的全部诊断（error 与 warn，P.09 K262 ④）
	Proposal string            // 非空：已转为提议
}

// applier 在 m 上应用一条 Op，可以补全 op.Payload（E.05-R7）。前置条件不满足时什么都不改，返回 ErrInvalid。
type applier func(m *mutable, op *gen.Op, e *env) (inverse []gen.Op, touched schema.Touched, err error)

// 注册点（E.05-R8）：后面的叶子在 init 里登记。
var (
	appliers  = map[gen.OpKind]applier{}
	expanders = map[gen.OpKind]hook{} // revert_op、revert_to 的展开（E.05.02）
	// coalescer 是拖动合并（E.05.04）：处理了就返回 handled，post 在事务提交后调用。
	coalescer func(ctx context.Context, p *Pipeline, r Repo, c *cached, b *Batch) (handled bool, res *Result, post func(), err error)
	// filterMissed 去掉互相抵消的记录再判断能否交换（E.05.02 §5.6）。
	filterMissed func(b *Batch, missed []gen.OpRecord) []gen.OpRecord
)

type env struct {
	view       schema.View
	newID      func() string
	internal   bool
	replay     bool // 重放日志：不补全、不生成 ID
	migrations []migrate.Step
}

func (p *Pipeline) env(internal, replay bool) *env {
	return &env{view: p.o.View, newID: p.o.NewID, internal: internal, replay: replay, migrations: p.o.Migrations}
}

func (p *Pipeline) applyOne(m *mutable, op *gen.Op, e *env) ([]gen.Op, schema.Touched, error) {
	a, ok := appliers[op.Kind]
	if !ok {
		return nil, schema.Touched{}, badf("%s 还不支持", op.Kind)
	}
	return a(m, op, e)
}

func payloadOf[T any](op *gen.Op) (*T, error) {
	v, err := decode[T](op)
	if err != nil {
		return nil, fmt.Errorf("%w：%v", ErrBadRequest, err)
	}
	return v, nil
}

func check[T any](op *gen.Op) error {
	_, err := decode[T](op)
	return err
}

// payloadChecks：M1 收的 kind 和严格解码（E.05-R6、P.09 K262 ①）。restore_nodes 只能作为逆操作，单独处理。
var payloadChecks = map[gen.OpKind]func(*gen.Op) error{
	gen.OpAddNode: check[gen.PayloadAddNode], gen.OpRemoveNodes: check[gen.PayloadRemoveNodes],
	gen.OpConnect: check[gen.PayloadConnect], gen.OpDisconnect: check[gen.PayloadDisconnect],
	gen.OpSetParam: check[gen.PayloadSetParam], gen.OpPromoteParam: check[gen.PayloadPromoteParam],
	gen.OpDemoteParam: check[gen.PayloadDemoteParam], gen.OpSetLabel: check[gen.PayloadSetLabel],
	gen.OpSetTags: check[gen.PayloadSetTags], gen.OpSetLock: check[gen.PayloadSetLock],
	gen.OpMoveNodes: check[gen.PayloadMoveNodes], gen.OpSetNodeUI: check[gen.PayloadSetNodeUi],
	gen.OpAddNote: check[gen.PayloadAddNote], gen.OpUpdateNote: check[gen.PayloadUpdateNote],
	gen.OpRemoveNotes: check[gen.PayloadRemoveNotes], gen.OpSetGraphTitle: check[gen.PayloadSetGraphTitle],
	gen.OpSetGraphMeta: check[gen.PayloadSetGraphMeta], gen.OpRevertOp: check[gen.PayloadRevertOp],
	gen.OpRevertTo: check[gen.PayloadRevertTo], gen.OpMigrate: check[gen.PayloadMigrate],
}

// laterKinds：还没收的 kind 和支持它的里程碑（E.05-R6）。
var laterKinds = map[gen.OpKind]string{
	gen.OpGroup:          "打包成子图要到 M2 才支持",
	gen.OpUngroup:        "解散子图要到 M2 才支持",
	gen.OpSetSubgraphRef: "切换子图的内联和引用要到 M2 才支持",
	gen.OpImportApply:    "导入要到 M2 才支持",
	gen.OpSetMandate:     "挂授权书要到 M3 才支持",
	gen.OpApplyPatch:     "应用模型补丁要到 M4 才支持",
	gen.OpCreateBranch:   "草稿分支要到 M4 才支持",
	gen.OpDeleteBranch:   "草稿分支要到 M4 才支持",
	gen.OpMergeBranch:    "合并草稿分支要到 M4 才支持",
}

// viewOnly：改了也不影响校验结果的 Op（E.05.01 §5.1 第 7 步）。只有 move_nodes：坐标在 graph.schema.json 里只要求是整数（解码时已查），
// 别的校验规则也不看坐标。set_node_ui（宽度、颜色）、便签（大小、字数、颜色）、标题和说明（字数）都受文档结构约束（V01），要校验（P.09 K263）。
var viewOnly = map[gen.OpKind]bool{gen.OpMoveNodes: true}

// blockingCodes：新出现时阻止提交的诊断，对应 C.05.02 的前置条件（P.09 K262 ②）。
var blockingCodes = map[gen.DiagCode]bool{
	gen.DiagV01: true, gen.DiagV03: true, gen.DiagV05: true, gen.DiagV06: true,
	gen.DiagV07: true, gen.DiagV09: true, gen.DiagV12: true, gen.DiagV14: true,
}

// Submit 提交一批 Op（E.05.01 §5.1）。
func (p *Pipeline) Submit(ctx context.Context, b Batch) (*Result, error) {
	return p.submit(ctx, b)
}

func (p *Pipeline) submit(ctx context.Context, b Batch) (*Result, error) {
	b.Ops = append([]gen.Op(nil), b.Ops...)
	if err := p.prepare(&b); err != nil {
		return nil, err
	}
	lk := p.lockOf(b.GraphID)
	lk.Lock()
	defer lk.Unlock()
	var res *Result
	var post func()
	err := p.o.Repo.Tx(ctx, func(r Repo) error {
		var err error
		res, post, err = p.submitTx(ctx, r, &b)
		return err
	})
	if err != nil {
		return nil, err
	}
	if post != nil {
		post()
	}
	return res, nil
}

func (p *Pipeline) lockOf(graphID string) *sync.Mutex {
	p.mu.Lock()
	defer p.mu.Unlock()
	l := p.locks[graphID]
	if l == nil {
		l = &sync.Mutex{}
		p.locks[graphID] = l
	}
	return l
}

// prepare 是第 0 步里不读库的部分。
func (p *Pipeline) prepare(b *Batch) error {
	if b.GraphID == "" || b.Branch == "" {
		return badf("没有写图或分支")
	}
	if b.pre != nil {
		return nil
	}
	if len(b.Ops) == 0 {
		return badf("一批至少要有一条 Op")
	}
	if len(b.Ops) > 500 {
		return badf("一批最多 500 条 Op，这一批有 %d 条", len(b.Ops))
	}
	for i := range b.Ops {
		o := &b.Ops[i]
		if o.Kind == gen.OpRevertOp || o.Kind == gen.OpRevertTo {
			if len(b.Ops) != 1 {
				return badf("第 %d 条：%s 只能单独提交", i+1, o.Kind)
			}
			if expanders[o.Kind] == nil {
				return badf("%s 还不支持", o.Kind)
			}
		}
		if b.internal {
			continue
		}
		if msg, ok := laterKinds[o.Kind]; ok {
			return badf("第 %d 条：%s", i+1, msg)
		}
		if o.Kind == gen.OpRestoreNodes {
			return badf("第 %d 条：restore_nodes 只能作为逆操作出现", i+1)
		}
		chk, ok := payloadChecks[o.Kind]
		if !ok {
			return badf("第 %d 条：不认识的 kind %q", i+1, o.Kind)
		}
		if err := chk(o); err != nil {
			return badf("第 %d 条：%v", i+1, err)
		}
		if b.Actor.Kind == gen.ActorKindModel && (o.Reason == nil || strings.TrimSpace(*o.Reason) == "") {
			return badf("第 %d 条：模型提交的 Op 要写 reason", i+1)
		}
	}
	return nil
}

// finishOps 是第 0 步里要读库的部分：op id、group、丢掉客户端填的服务器字段、taint、授权书。
func (p *Pipeline) finishOps(ctx context.Context, r Repo, b *Batch) error {
	if len(b.Ops) == 0 {
		return badf("一批至少要有一条 Op")
	}
	group := b.GroupID
	for i := 0; group == "" && i < len(b.Ops); i++ {
		if b.Ops[i].GroupID != nil {
			group = string(*b.Ops[i].GroupID)
		}
	}
	if group == "" {
		group = p.o.NewID()
	}
	if !validID(group) {
		return badf("group_id %q 不是 ULID", group)
	}
	seen := map[string]bool{}
	for i := range b.Ops {
		o := &b.Ops[i]
		if o.GroupID != nil && string(*o.GroupID) != group && !b.internal {
			return badf("第 %d 条的 group_id 和这一批的不同", i+1)
		}
		if o.ID == nil {
			id := gen.Ulid(p.o.NewID())
			o.ID = &id
		} else if id := string(*o.ID); !validID(id) {
			return badf("第 %d 条的 id %q 不是 ULID", i+1, id)
		} else if !b.internal {
			if _, err := r.Op(ctx, id); err == nil {
				return badf("第 %d 条的 id %s 已经用过", i+1, id)
			} else if !errors.Is(err, ErrRepoNotFound) {
				return err
			}
		}
		if seen[string(*o.ID)] {
			return badf("这一批里 id %s 重复", *o.ID)
		}
		seen[string(*o.ID)] = true
		g := gen.Ulid(group)
		o.GroupID = &g
		o.Actor, o.Approver, o.Inverse, o.Ts = nil, nil, nil, nil
		if b.Taint || (o.Taint != nil && *o.Taint) {
			o.Taint = ptr(true) // 只能从 false 变 true（C.05.01 §3）
		} else {
			o.Taint = nil
		}
		if b.MandateID != "" {
			mid := gen.Ulid(b.MandateID)
			o.MandateID = &mid
		}
	}
	b.GroupID = group
	return nil
}

func joinPosts(fs ...func()) func() {
	return func() {
		for _, f := range fs {
			if f != nil {
				f()
			}
		}
	}
}

// submitTx 是一个事务里的第 2–8 步。
func (p *Pipeline) submitTx(ctx context.Context, r Repo, b *Batch) (*Result, func(), error) {
	k := cacheKey{b.GraphID, b.Branch}
	head, err := r.Head(ctx, b.GraphID, b.Branch)
	if err != nil {
		return nil, nil, repoErr(err, fmt.Sprintf("图 %s 分支 %s", b.GraphID, b.Branch))
	}
	c, err := p.headState(ctx, r, k, head)
	if err != nil {
		return nil, nil, err
	}
	var posts []func()
	if from, to, ok := p.migration(c.doc.SchemaVersion); ok && !b.migrating {
		mb := Batch{GraphID: b.GraphID, Branch: b.Branch, BaseVersion: head,
			Actor: gen.Actor{Kind: gen.ActorKindRuntime, ID: "migrate"}, internal: true, migrating: true,
			Ops: []gen.Op{mk(gen.OpMigrate, gen.PayloadMigrate{FromSchema: from, ToSchema: to})}}
		mb.Ops[0].Reason = ptr("格式升级")
		if err := p.finishOps(ctx, r, &mb); err != nil {
			return nil, nil, err
		}
		_, post, nc, err := p.applyBatch(ctx, r, c, &mb, false)
		if err != nil {
			return nil, nil, fmt.Errorf("op: 格式升级 %s → %s 失败：%w", from, to, err)
		}
		posts = append(posts, post)
		if b.BaseVersion == head {
			b.BaseVersion = nc.version // 自动插进去的升级不算客户端错过的修改
		}
		head, c = nc.version, nc
	}
	pre := b.pre
	if pre == nil && len(b.Ops) == 1 {
		pre = expanders[b.Ops[0].Kind]
	}
	if pre != nil {
		if err := pre(ctx, p, r, c, b); err != nil {
			return nil, nil, err
		}
	}
	if err := p.finishOps(ctx, r, b); err != nil {
		return nil, nil, err
	}
	if b.BaseVersion < 0 || b.BaseVersion > head {
		return nil, nil, badf("base_version %d 不在 0–%d 之间", b.BaseVersion, head)
	}
	rebased := false
	if b.BaseVersion < head {
		missed, err := r.Range(ctx, b.GraphID, b.Branch, b.BaseVersion, head)
		if err != nil {
			return nil, nil, repoErr(err, "错过的 Op")
		}
		cmp := missed
		if filterMissed != nil {
			cmp = filterMissed(b, missed)
		}
		if ok, cs := Commutes(b.Ops, c.g, p.o.View, cmp); !ok {
			return nil, nil, &ConflictError{Head: head, Missed: missed, Conflicts: cs}
		}
		b.Ops = Rebase(b.Ops, cmp)
		rebased = true
		if len(b.Ops) == 0 {
			res := &Result{Version: head, Applied: []string{}, Rebased: true, IDMap: map[string]string{}, Diags: append([]gen.Diag{}, c.diags...)}
			return res, joinPosts(posts...), nil
		}
	}
	if coalescer != nil && !b.internal {
		handled, res, post, err := coalescer(ctx, p, r, c, b)
		if err != nil {
			return nil, nil, err
		}
		if handled {
			return res, joinPosts(append(posts, post)...), nil
		}
	}
	res, post, _, err := p.applyBatch(ctx, r, c, b, rebased)
	if err != nil {
		return nil, nil, err
	}
	return res, joinPosts(append(posts, post)...), nil
}

// migration：文档的格式能沿迁移链升级到哪一版（C.12-R2、E.05.01 §5.5）。
func (p *Pipeline) migration(ver string) (from, to string, ok bool) {
	to = ver
	for range p.o.Migrations {
		next := ""
		for _, s := range p.o.Migrations {
			if s.From == to {
				next = s.To
				break
			}
		}
		if next == "" {
			break
		}
		to = next
	}
	return ver, to, to != ver
}

// applyBatch 是第 5–8 步：作用对象与授权 → 在副本上应用 → 校验 → 落库。返回事务提交后要做的事和新的状态。
func (p *Pipeline) applyBatch(ctx context.Context, r Repo, c *cached, b *Batch, rebased bool) (*Result, func(), *cached, error) {
	for i := range b.Ops {
		o := &b.Ops[i]
		t, err := Target(o, c.g)
		if err != nil {
			return nil, nil, nil, badf("第 %d 条：%v", i+1, err)
		}
		if o.Target != nil && p.o.Logger != nil && string(mustRaw(o.Target)) != string(mustRaw(t)) {
			p.o.Logger.Warn("客户端填的 target 和服务器算的不同，以服务器的为准", "op", *o.ID, "kind", o.Kind)
		}
		o.Target = &t
	}
	d, results, err := authorize(ctx, p.o.Authz, p.o.Estop, authInput{Actor: b.Actor, Approver: b.Approver, Taint: b.Taint,
		Mandate: b.MandateID, RunID: b.RunID, Ops: b.Ops, G: c.g})
	if err != nil {
		return nil, nil, nil, err
	}
	if d == Propose {
		if p.o.Proposer == nil {
			return nil, nil, nil, &DeniedError{Err: ErrForbidden, Reasons: []string{"M1 还不能转为提议"}}
		}
		id, err := p.o.Proposer.Propose(ctx, r, *b, results)
		if err != nil {
			return nil, nil, nil, err
		}
		return &Result{Version: c.version, Applied: []string{}, Rebased: rebased, IDMap: map[string]string{},
			Diags: append([]gen.Diag{}, c.diags...), Proposal: id}, nil, c, nil
	}

	m := newMutable(cloneDoc(c.doc))
	e := p.env(b.internal, false)
	var touched schema.Touched
	fullCheck, onlyView := false, true
	for i := range b.Ops {
		o := &b.Ops[i]
		inv, t, err := p.applyOne(m, o, e)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("第 %d 条（%s）：%w", i+1, o.Kind, err)
		}
		m.doc.Version++
		if o.Kind == gen.OpMigrate {
			fullCheck = true // 不可逆，不写 inverse（C.05.03 §2）
		} else {
			if inv == nil {
				inv = []gen.Op{}
			}
			o.Inverse = &inv
		}
		touched.Nodes = append(touched.Nodes, t.Nodes...)
		touched.Edges = append(touched.Edges, t.Edges...)
		if !viewOnly[o.Kind] {
			onlyView = false
		}
	}

	ng, err := graph.FromDoc(m.doc)
	if err != nil {
		return nil, nil, nil, invalidf("%v", err)
	}
	opts := schema.Opts{ForRun: true}
	var diags []gen.Diag
	switch {
	case fullCheck:
		diags = schema.Validate(ng, p.o.View, opts)
	case onlyView:
		diags = c.diags
	default:
		diags = schema.ValidateIncremental(ng, c.diags, touched, p.o.View, opts)
	}
	diags = nonNil(diags)
	if !b.migrating {
		if bl := blocking(c.diags, diags); len(bl) > 0 {
			return nil, nil, nil, &InvalidGraphError{Diags: bl}
		}
	}

	last := m.doc.Version
	now := p.o.Now().UTC()
	ts := gen.Timestamp{Time: now}
	gid, br, base := gen.Ulid(b.GraphID), gen.Branch(b.Branch), b.BaseVersion
	recs := make([]gen.OpRecord, len(b.Ops))
	applied := make([]string, len(b.Ops))
	for i := range b.Ops {
		o := b.Ops[i]
		actor := b.Actor
		o.GraphID, o.Branch, o.BaseVersion, o.Actor, o.Approver, o.Ts = &gid, &br, &base, &actor, b.Approver, &ts
		recs[i] = gen.OpRecord{GraphID: gid, Branch: br, Op: o, Version: c.version + int64(i) + 1}
		applied[i] = string(*o.ID)
	}
	var fullDoc *gen.Graph
	lastFull := c.lastFull
	if last-c.lastFull >= fullEvery && b.Ops[len(b.Ops)-1].CoalesceKey == nil {
		fullDoc, lastFull = cloneDoc(m.doc), last
	}
	title := ""
	if m.doc.Title != nil {
		title = *m.doc.Title
	}
	if err := r.Commit(ctx, Commit{GraphID: b.GraphID, Branch: b.Branch, Ops: recs, Doc: fullDoc,
		Hash: graph.Hash(m.doc), Title: title, Stats: statsOf(ng), At: now}); err != nil {
		if errors.Is(err, ErrRepoConflict) {
			return nil, nil, nil, &ConflictError{Head: c.version, Reason: "存储报告版本冲突：" + err.Error()}
		}
		return nil, nil, nil, repoErr(err, "提交")
	}
	if err := p.pushUndo(ctx, r, b); err != nil {
		return nil, nil, nil, err
	}
	res := &Result{Version: last, Applied: applied, Rebased: rebased, IDMap: map[string]string{}, Diags: append([]gen.Diag{}, diags...)}
	if b.onTx != nil {
		if err := b.onTx(ctx, r, res); err != nil {
			return nil, nil, nil, err
		}
	}
	nc := &cached{version: last, doc: m.doc, g: ng, diags: diags, lastFull: lastFull, lastCoalesce: time.Time{}} // 普通提交之后不再接着合并
	k := cacheKey{b.GraphID, b.Branch}
	evs := events(b.Actor, recs, last, diags, ts)
	post := func() {
		p.cachePut(k, nc)
		if p.o.Bus != nil {
			p.o.Bus.Publish(evs)
		}
		if p.o.Runs != nil {
			p.o.Runs.GraphChanged(b.GraphID, b.Branch, last)
		}
	}
	return res, post, nc, nil
}

// events：每条 Op 一个 op.applied（seq 为版本号），再一条 validate（E 级，seq 0，C.06.03、P.09 K262 ④）。
func events(actor gen.Actor, recs []gen.OpRecord, last int64, diags []gen.Diag, ts gen.Timestamp) []gen.GraphEvent {
	evs := make([]gen.GraphEvent, 0, len(recs)+1)
	for _, rec := range recs {
		evs = append(evs, gen.GraphEvent{Actor: actor, GraphID: rec.GraphID, Kind: gen.EvOpApplied, Seq: rec.Version, Ts: ts,
			Data: mustRaw(gen.DataOpApplied{Op: rec.Op, Version: rec.Version})})
	}
	if len(recs) > 0 {
		evs = append(evs, gen.GraphEvent{Actor: actor, GraphID: recs[0].GraphID, Kind: gen.EvValidate, Seq: 0, Ts: ts,
			Data: mustRaw(gen.DataValidate{Diags: mustRaw(nonNil(diags)), Version: last})})
	}
	return evs
}

func deref[T any](p *T) string {
	if p == nil {
		return ""
	}
	return fmt.Sprint(*p)
}

func diagKey(d gen.Diag) string {
	return string(d.Code) + "|" + deref(d.Node) + "|" + deref(d.Port) + "|" + deref(d.Param) + "|" + deref(d.Scope)
}

// blocking：cur 里新出现的、阻止提交的 error（按码、节点、端口、参数、层比较，不比文案）。
func blocking(prev, cur []gen.Diag) []gen.Diag {
	old := make(map[string]bool, len(prev))
	for _, d := range prev {
		old[diagKey(d)] = true
	}
	var out []gen.Diag
	for _, d := range cur {
		if d.Level == gen.DiagLevelError && blockingCodes[d.Code] && !old[diagKey(d)] {
			out = append(out, d)
		}
	}
	return out
}

// redoSuffix：重做栈的 ActorID 是 <actor.id>#redo（E.05.02 §5.1）。
const redoSuffix = "#redo"

// pushUndo 是第 8 步的撤销栈：normal、redo 批次推进撤销栈（栈顶已是这个 group 时不重复推）；normal 批次清空重做栈。
func (p *Pipeline) pushUndo(ctx context.Context, r Repo, b *Batch) error {
	if b.mode == modeUndo || b.migrating {
		return nil
	}
	if b.Actor.Kind != gen.ActorKindUser && b.Actor.Kind != gen.ActorKindModel {
		return nil
	}
	id := b.Actor.ID
	top, ok, err := r.UndoTop(ctx, b.GraphID, b.Branch, id, false)
	if err != nil {
		return err
	}
	if !ok || top.GroupID != b.GroupID {
		seq, err := nextSeq(ctx, r, b.GraphID, b.Branch, id)
		if err != nil {
			return err
		}
		if err := r.UndoPush(ctx, UndoEntry{GraphID: b.GraphID, Branch: b.Branch, ActorID: id, Seq: seq, GroupID: b.GroupID}); err != nil {
			return err
		}
	}
	if b.mode == modeRedo {
		return nil // 重做不清空重做栈（E.05.02 §5.4）
	}
	for {
		top, ok, err := r.UndoTop(ctx, b.GraphID, b.Branch, id+redoSuffix, false)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := r.UndoMark(ctx, top, true); err != nil {
			return err
		}
	}
}

// nextSeq 是这个栈（两种状态都算）最大的 Seq + 1。
func nextSeq(ctx context.Context, r Repo, graphID, branch, actorID string) (int64, error) {
	a, _, err := r.UndoTop(ctx, graphID, branch, actorID, false)
	if err != nil {
		return 0, err
	}
	d, _, err := r.UndoTop(ctx, graphID, branch, actorID, true)
	if err != nil {
		return 0, err
	}
	return max(a.Seq, d.Seq) + 1, nil
}
