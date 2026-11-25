package schema

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mio-cc/anywhere-door/gen"
)

// V01 文档结构合法：按 graph.schema.json 校验整份文档。
var ruleV01 = rule{code: gen.DiagV01, check: func(c *Context) []gen.Diag {
	bad := func(detail string) []gen.Diag {
		return []gen.Diag{c.diag(gen.DiagV01, gen.DiagLevelError, Loc{}, map[string]any{"detail": detail})}
	}
	sch, ok := c.V.DocSchema("graph.schema.json")
	if !ok {
		return bad("注册表里没有 graph.schema.json")
	}
	b, err := json.Marshal(c.G.Doc())
	if err != nil {
		return bad(err.Error())
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return bad(err.Error())
	}
	if err := sch.Validate(inst); err != nil {
		var ve *jsonschema.ValidationError
		if !errors.As(err, &ve) {
			return bad(err.Error())
		}
		loc, msg := firstLeaf(ve)
		if p := pointer(loc); p != "" {
			msg = p + "：" + msg
		}
		return bad(msg)
	}
	return nil
}}
