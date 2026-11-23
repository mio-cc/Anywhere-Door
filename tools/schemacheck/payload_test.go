package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const opVectorsFile = "schema/examples/op_vectors.json"

// opVector 是 op_vectors.json 的一项（C.05.02 实施卡的向量格式）。
type opVector struct {
	Name    string          `json:"name"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
	Valid   *bool           `json:"valid"`
	ErrorAt *string         `json:"error_at"`
}

// payloadDef 是 kind 对应的 payload 定义名：Payload 加 kind 的驼峰写法（add_node → PayloadAddNode，C.05.02 §6.1）。
func payloadDef(kind string) string {
	var b strings.Builder
	b.WriteString("Payload")
	for _, w := range strings.Split(kind, "_") {
		if w != "" {
			b.WriteString(strings.ToUpper(w[:1]) + w[1:])
		}
	}
	return b.String()
}

// readOpVectors 读 op_vectors.json；不认识的字段报错。
func readOpVectors(t *testing.T) []opVector {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(opVectorsFile)))
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var vs []opVector
	if err := dec.Decode(&vs); err != nil {
		t.Fatalf("%s：%v", opVectorsFile, err)
	}
	return vs
}

// opAllOf 读 Op 的 allOf：kind → then 里 payload 的 $ref；一种 kind 出现两次时报错。
func opAllOf(t *testing.T, s *Set) map[string]string {
	t.Helper()
	defs, _ := s.Raw["op.schema.json"]["$defs"].(map[string]any)
	op, _ := defs["Op"].(map[string]any)
	all, ok := op["allOf"].([]any)
	if !ok {
		t.Fatal("op.schema.json 的 Op 没有 allOf")
	}
	out := map[string]string{}
	for i, item := range all {
		m, _ := item.(map[string]any)
		ifs, _ := m["if"].(map[string]any)
		ifProps, _ := ifs["properties"].(map[string]any)
		kindSch, _ := ifProps["kind"].(map[string]any)
		kind, _ := kindSch["const"].(string)
		thens, _ := m["then"].(map[string]any)
		thenProps, _ := thens["properties"].(map[string]any)
		paySch, _ := thenProps["payload"].(map[string]any)
		ref, _ := paySch["$ref"].(string)
		req, _ := ifs["required"].([]any)
		if kind == "" || ref == "" || len(req) != 1 || req[0] != "kind" {
			t.Errorf("Op.allOf[%d] 不是 {if: {required: [kind], properties: {kind: {const}}}, then: {properties: {payload: {$ref}}}}", i)
			continue
		}
		if _, dup := out[kind]; dup {
			t.Errorf("Op.allOf 里 %s 出现了两次", kind)
		}
		out[kind] = ref
	}
	return out
}

func TestPayload(t *testing.T) {
	t.Run("C0502-T1 payload 向量", func(t *testing.T) {
		s := repoSet(t)
		kinds := opKindEnum(t, s)
		known := map[string]bool{}
		for _, k := range kinds {
			known[k] = true
		}
		valid, invalid, names := map[string]int{}, map[string]int{}, map[string]bool{}
		for i, v := range readOpVectors(t) {
			switch {
			case v.Name == "" || names[v.Name]:
				t.Errorf("第 %d 项：name 为空或重复：%q", i, v.Name)
			case !known[v.Kind]:
				t.Errorf("%s：kind %q 不在 OpKind 里", v.Name, v.Kind)
			case len(v.Payload) == 0 || v.Valid == nil:
				t.Errorf("%s：要写 payload 和 valid", v.Name)
			case *v.Valid && v.ErrorAt != nil:
				t.Errorf("%s：valid 为 true 时不写 error_at", v.Name)
			case !*v.Valid && v.ErrorAt == nil:
				t.Errorf("%s：valid 为 false 时要写 error_at", v.Name)
			}
			names[v.Name] = true
			if v.Valid == nil || len(v.Payload) == 0 {
				continue
			}
			text := `{"kind": "` + v.Kind + `", "payload": ` + string(v.Payload) + `}`
			if *v.Valid {
				valid[v.Kind]++
				mustValid(t, s, opRef, text)
			} else if v.ErrorAt != nil {
				invalid[v.Kind]++
				mustInvalid(t, s, opRef, text, *v.ErrorAt)
			}
		}
		for _, k := range kinds {
			if valid[k] == 0 || invalid[k] == 0 {
				t.Errorf("%s：合法 %d 个、不合法 %d 个，每种至少各一个", k, valid[k], invalid[k])
			}
		}
	})
	t.Run("C0502-T4 判别", func(t *testing.T) {
		s := repoSet(t)
		kinds := opKindEnum(t, s)
		refs := opAllOf(t, s)
		var want []string
		for _, k := range kinds {
			want = append(want, payloadDef(k))
			if got := refs[k]; got != "#/$defs/"+payloadDef(k) {
				t.Errorf("Op.allOf 里 %s 的 payload 应当引用 #/$defs/%s，实际是 %q", k, payloadDef(k), got)
			}
		}
		if len(refs) != len(kinds) {
			t.Errorf("Op.allOf 有 %d 种 kind，OpKind 有 %d 种", len(refs), len(kinds))
		}
		defs, _ := s.Raw["op.schema.json"]["$defs"].(map[string]any)
		var got []string
		for name := range defs {
			if strings.HasPrefix(name, "Payload") {
				got = append(got, name)
			}
		}
		if !sameSet(got, want) {
			t.Errorf("Payload… 定义是 %v，应当和 OpKind 一一对应：%v", got, want)
		}

		const node = `{"id": "` + opID1 + `", "type": "Agent", "pos": {"x": 0, "y": 0}}`
		mustValid(t, s, opRef, `{"kind": "add_node", "payload": {"scope": "root", "node": `+node+`}}`)
		mustInvalid(t, s, opRef, `{"kind": "add_note", "payload": {"scope": "root", "node": `+node+`}}`, "/payload")
		mustInvalid(t, s, opRef, `{"kind": "remove_nodes", "payload": {"ids": ["`+opID1+`"], "x_extra": 1}}`, "/payload")
		mustInvalid(t, s, opRef, `{"kind": "remove_node", "payload": {"ids": ["`+opID1+`"]}}`, "/kind")
		const resRef = "op.schema.json#/$defs/Resolution"
		mustValid(t, s, resRef, `{"object": {"node": "`+opID1+`", "field": "label"}, "pick": "ours"}`)
		mustInvalid(t, s, resRef, `{"object": {}, "pick": "ours"}`, "/object")
	})
}
