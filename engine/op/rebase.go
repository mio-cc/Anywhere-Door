package op

import (
	"encoding/json"

	"github.com/mio-cc/anywhere-door/gen"
)

// Rebase 把这一批 Op 改写到最新版本上（E.05.03 §5.4），只在 Commutes 为 true 之后调用。ID 都是 ULID，不用改写；
// remove_nodes 去掉已被 missed 删掉的节点，删空的整条去掉。整批都去掉时返回空切片。
func Rebase(ops []gen.Op, missed []gen.OpRecord) []gen.Op {
	gone := map[gen.Ulid]bool{}
	for i := range missed {
		m := &missed[i].Op
		if m.Kind != gen.OpRemoveNodes {
			continue
		}
		if p, err := decode[gen.PayloadRemoveNodes](m); err == nil {
			for _, id := range p.IDs {
				gone[id] = true
			}
		}
		inv := inverseOps(m)
		for j := range inv {
			if inv[j].Kind != gen.OpRestoreNodes {
				continue
			}
			if p, err := decode[gen.PayloadRestoreNodes](&inv[j]); err == nil {
				for k := range p.Removed.Nodes {
					for _, id := range nodeTree(&p.Removed.Nodes[k].Node) {
						gone[gen.Ulid(id)] = true
					}
				}
			}
		}
	}
	out := make([]gen.Op, 0, len(ops))
	for _, op := range ops {
		if op.Kind == gen.OpRemoveNodes && len(gone) > 0 {
			if p, err := decode[gen.PayloadRemoveNodes](&op); err == nil {
				keep := make([]gen.Ulid, 0, len(p.IDs))
				for _, id := range p.IDs {
					if !gone[id] {
						keep = append(keep, id)
					}
				}
				if len(keep) == 0 {
					continue
				}
				if len(keep) != len(p.IDs) {
					p.IDs = keep
					if raw, err := json.Marshal(p); err == nil {
						op.Payload = raw
					}
				}
			}
		}
		out = append(out, op)
	}
	return out
}
