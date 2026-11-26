// Package op 是 Op 引擎（设计文档 E.05）：修改图的唯一入口（INV-08）。
// 流水线、撤销重做、可交换判定与 rebase、拖动合并、授权钩子都在这个包里。
package op

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mio-cc/anywhere-door/gen"
)

// decode 按 kind 解出 payload（gen.DecodeAs 的包装，E.05.03 §4）：不认识的字段报错，错误里写 kind。
func decode[T any](op *gen.Op) (*T, error) {
	v, err := gen.DecodeAs[T](op.Payload)
	if err != nil {
		return nil, fmt.Errorf("op: %s 的 payload 解不开：%w", op.Kind, err)
	}
	return v, nil
}

// nodeOf 取 payload 里的 node 字段（set_label、set_tags、set_lock、set_node_ui 这类只改一个节点的 Op）。
func nodeOf(op *gen.Op) (string, error) {
	var p struct {
		Node gen.Ulid `json:"node"`
	}
	if err := json.Unmarshal(op.Payload, &p); err != nil || p.Node == "" {
		return "", fmt.Errorf("op: %s 的 payload 没有 node", op.Kind)
	}
	return string(p.Node), nil
}

// inverseOps 是一条 Op 的逆操作；没有时为空。
func inverseOps(op *gen.Op) []gen.Op {
	if op.Inverse == nil {
		return nil
	}
	return *op.Inverse
}

// sortedIDs 去重、排序。
func sortedIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func ulids(ids []string) []gen.Ulid {
	out := make([]gen.Ulid, len(ids))
	for i, id := range ids {
		out[i] = gen.Ulid(id)
	}
	return out
}

func strs(ids []gen.Ulid) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

// nodeTree 是节点和它子图里全部节点的 ID（先自己，再按文档顺序深度优先）。
func nodeTree(n *gen.Node) []string {
	out := []string{string(n.ID)}
	if n.Children != nil {
		for i := range n.Children.Nodes {
			out = append(out, nodeTree(&n.Children.Nodes[i])...)
		}
	}
	return out
}
