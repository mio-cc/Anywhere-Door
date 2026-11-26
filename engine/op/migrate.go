package op

import (
	"encoding/json"

	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/engine/schema/migrate"
	"github.com/mio-cc/anywhere-door/gen"
)

// migrate Op：文档格式升级（C.12-R2、E.05.01 §5.5）。不可逆，没有 inverse；流水线对它做全量校验。
func init() {
	appliers[gen.OpMigrate] = applyMigrate
}

func applyMigrate(m *mutable, op *gen.Op, e *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadMigrate](op)
	if err != nil {
		return nil, noTouch, err
	}
	if p.FromSchema != m.doc.SchemaVersion {
		return nil, noTouch, invalidf("文档格式是 %s，不是 %s", m.doc.SchemaVersion, p.FromSchema)
	}
	raw, err := json.Marshal(m.doc)
	if err != nil {
		return nil, noTouch, invalidf("%v", err)
	}
	out, _, err := migrate.Apply(raw, e.migrations)
	if err != nil {
		return nil, noTouch, invalidf("%v", err)
	}
	var d gen.Graph
	if err := json.Unmarshal(out, &d); err != nil {
		return nil, noTouch, invalidf("升级后的文档读不出来：%v", err)
	}
	if d.SchemaVersion != p.ToSchema {
		return nil, noTouch, invalidf("升级到的是 %s，不是 %s", d.SchemaVersion, p.ToSchema)
	}
	d.Version = m.doc.Version
	normalizeDoc(&d, e.view)
	*m.doc = d
	m.reindex()
	return nil, noTouch, nil
}
