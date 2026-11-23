// Package memstore 是完全在内存里的 store.Store（E.07.05），只用于测试。
// 每条记录按 JSON 编码后保存，读出时解码，读出的值不和调用方共享内存。
package memstore

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/gen"
)

// data 是全部集合；值都是不可变的（替换而不是原地修改），所以复制时浅拷贝每个 map 即可。
type data struct {
	graphs     map[string][]byte     // 图 id → GraphMeta
	branches   map[string][]byte     // 图/分支 → BranchMeta（behind、ahead 存 0）
	versions   map[string]versionRow // 图/分支/版本 → 哈希和全文
	ops        map[string][]byte     // 图/分支/版本 → OpRecord
	opKeys     map[string]string     // Op id → ops 的键
	undo       map[string]store.UndoEntry
	runs       map[string][]byte // 运行 id → RunMeta
	leases     map[string]lease
	runEvents  map[string][][]byte // 运行 id → 事件，下标是 seq-1
	sessEvents map[string][][]byte
	nodeStates map[string][]byte // 运行/exec_key → NodeStateRow
	views      map[string]string // 运行/seq → 视图引用
	cps        map[string][]byte // 运行/exec_key → Checkpoint
	cpHist     map[string][]byte // 运行/exec_key/seq → Checkpoint
	tools      map[string][]byte
	settings   map[string][]byte
}

type versionRow struct {
	Hash string
	Doc  []byte // 只有存全文的版本有
}

type lease struct {
	owner string
	until time.Time
}

func newData() *data {
	return &data{
		graphs: map[string][]byte{}, branches: map[string][]byte{}, versions: map[string]versionRow{},
		ops: map[string][]byte{}, opKeys: map[string]string{}, undo: map[string]store.UndoEntry{},
		runs: map[string][]byte{}, leases: map[string]lease{}, runEvents: map[string][][]byte{},
		sessEvents: map[string][][]byte{}, nodeStates: map[string][]byte{}, views: map[string]string{},
		cps: map[string][]byte{}, cpHist: map[string][]byte{}, tools: map[string][]byte{}, settings: map[string][]byte{},
	}
}

func clipAll(m map[string][][]byte) map[string][][]byte {
	out := make(map[string][][]byte, len(m))
	for k, v := range m {
		out[k] = slices.Clip(v) // 之后的 append 会重新分配，不写进原来的底层数组
	}
	return out
}

func (d *data) clone() *data {
	return &data{
		graphs: maps.Clone(d.graphs), branches: maps.Clone(d.branches), versions: maps.Clone(d.versions),
		ops: maps.Clone(d.ops), opKeys: maps.Clone(d.opKeys), undo: maps.Clone(d.undo),
		runs: maps.Clone(d.runs), leases: maps.Clone(d.leases), runEvents: clipAll(d.runEvents),
		sessEvents: clipAll(d.sessEvents), nodeStates: maps.Clone(d.nodeStates), views: maps.Clone(d.views),
		cps: maps.Clone(d.cps), cpHist: maps.Clone(d.cpHist), tools: maps.Clone(d.tools), settings: maps.Clone(d.settings),
	}
}

type memStore struct {
	mu *sync.Mutex
	d  *data
	tx bool // 在 Tx 里：已经持锁，d 是副本
}

// New 返回一个空的内存 Store。
func New() store.Store { return &memStore{mu: &sync.Mutex{}, d: newData()} }

func (s *memStore) Graphs() store.GraphRepo           { return graphs{s} }
func (s *memStore) Ops() store.OpRepo                 { return ops{s} }
func (s *memStore) Runs() store.RunRepo               { return runs{s} }
func (s *memStore) Events() store.EventRepo           { return events{s} }
func (s *memStore) Checkpoints() store.CheckpointRepo { return checkpoints{s} }
func (s *memStore) ToolCalls() store.ToolCallRepo     { return toolCalls{s} }
func (s *memStore) Settings() store.SettingsRepo      { return settings{s} }
func (s *memStore) Close() error                      { return nil }

// Tx 在全部数据的副本上执行 fn，成功时换成副本；嵌套调用直接执行 fn。
func (s *memStore) Tx(ctx context.Context, fn func(store.Store) error) (err error) {
	if s.tx {
		return fn(s)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.d.clone()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("store: Tx 里 panic：%v", r)
		}
	}()
	if err := fn(&memStore{mu: s.mu, d: c, tx: true}); err != nil {
		return err
	}
	*s.d = *c
	return nil
}

// write 在副本上执行 fn，成功时换成副本：单个方法要么全部写入，要么什么都不写。
func (s *memStore) write(fn func(d *data) error) error {
	if !s.tx {
		s.mu.Lock()
		defer s.mu.Unlock()
	}
	c := s.d.clone()
	if err := fn(c); err != nil {
		return err
	}
	*s.d = *c
	return nil
}

func (s *memStore) read(fn func(d *data) error) error {
	if !s.tx {
		s.mu.Lock()
		defer s.mu.Unlock()
	}
	return fn(s.d)
}

func enc(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("store: 编码失败：%w", err)
	}
	return b, nil
}

func dec[T any](b []byte) (T, error) {
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return v, fmt.Errorf("store: 解码失败：%w", err)
	}
	return v, nil
}

// key 用 \x00 连接各段。
func key(parts ...string) string { return strings.Join(parts, "\x00") }

// num 把版本号、seq 编成定长，按字符串排序就是按数字排序。
func num(n int64) string { return fmt.Sprintf("%020d", n) }

func prefix(parts ...string) string { return key(parts...) + "\x00" }

// lastNum 取键的最后一段（num 编码的数字）。
func lastNum(k string) int64 {
	n, _ := strconv.ParseInt(k[strings.LastIndexByte(k, 0)+1:], 10, 64)
	return n
}

func ts(t time.Time) gen.Timestamp { return gen.Timestamp{Time: t.UTC()} }

// sortKeyed 按 (时间, id) 倒序排，desc 为 false 时升序。
func sortKeyed[T any](items []T, at func(T) time.Time, id func(T) string, desc bool) {
	sort.Slice(items, func(i, j int) bool {
		ti, tj := at(items[i]), at(items[j])
		if !ti.Equal(tj) {
			return ti.After(tj) == desc
		}
		return (id(items[i]) > id(items[j])) == desc
	})
}

// page 对已按 (时间, id) 倒序排好的 items 做键集分页。
func page[T any](items []T, at func(T) time.Time, id func(T) string, cursor string, limit int) (store.Page[T], error) {
	start := 0
	if cursor != "" {
		ct, cid, err := store.DecodeCursor(cursor)
		if err != nil {
			return store.Page[T]{}, err
		}
		start = len(items)
		for i, it := range items {
			t := at(it).Truncate(time.Millisecond)
			if t.Before(ct) || (t.Equal(ct) && id(it) < cid) {
				start = i
				break
			}
		}
	}
	limit = store.ClampLimit(limit)
	end := min(start+limit, len(items))
	p := store.Page[T]{Items: append([]T{}, items[start:end]...)}
	if end < len(items) && end > start {
		last := items[end-1]
		p.NextCursor = store.EncodeCursor(at(last), id(last))
	}
	return p, nil
}
