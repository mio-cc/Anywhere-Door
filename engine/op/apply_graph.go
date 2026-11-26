package op

import (
	"encoding/json"
	"unicode/utf8"

	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// 图级（E.05.01 §5.2）：标题、说明。revert_to 的应用在 E.05.02（revert.go）。
func init() {
	appliers[gen.OpSetGraphTitle] = applySetGraphTitle
	appliers[gen.OpSetGraphMeta] = applySetGraphMeta
}

func applySetGraphTitle(m *mutable, op *gen.Op, e *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadSetGraphTitle](op)
	if err != nil {
		return nil, noTouch, err
	}
	if l := utf8.RuneCountInString(p.To); l > 120 || (l == 0 && !e.internal) {
		return nil, noTouch, invalidf("标题要 1–120 个字，现在是 %d 个", l)
	}
	old := ""
	if m.doc.Title != nil {
		old = *m.doc.Title
	}
	if p.To == "" {
		m.doc.Title = nil
	} else {
		m.doc.Title = ptr(p.To)
	}
	return []gen.Op{mk(gen.OpSetGraphTitle, gen.PayloadSetGraphTitle{To: old})}, noTouch, nil
}

func applySetGraphMeta(m *mutable, op *gen.Op, _ *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadSetGraphMeta](op)
	if err != nil {
		return nil, noTouch, err
	}
	var old gen.PayloadSetGraphMetaTo
	if m.doc.Meta != nil {
		if raw, ok := (*m.doc.Meta)["description"]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil {
				old.Description = &s
			}
		}
	}
	if p.To.Description == nil {
		if m.doc.Meta != nil {
			delete(*m.doc.Meta, "description")
		}
	} else {
		if m.doc.Meta == nil {
			m.doc.Meta = &gen.Meta{}
		}
		(*m.doc.Meta)["description"] = mustRaw(*p.To.Description)
	}
	if m.doc.Meta != nil && len(*m.doc.Meta) == 0 {
		m.doc.Meta = nil
	}
	return []gen.Op{mk(gen.OpSetGraphMeta, gen.PayloadSetGraphMeta{To: old})}, noTouch, nil
}
