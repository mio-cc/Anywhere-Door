package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree 在临时目录里写出一棵小仓库（每项是 相对路径、内容），返回根目录。
func writeTree(t *testing.T, files [][2]string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f[0]))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f[1]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// schemaDoc 拼一个写法合规的 schema 文件，body 是顶层除 $schema、$id、x-version 以外的部分。
func schemaDoc(name, body string) string {
	head := `{"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "https://anydoor.local/schema/` + name + `", "x-version": "1.0"`
	if strings.TrimSpace(body) == "" {
		return head + "}"
	}
	return head + ", " + body + "}"
}

// wantRules：problems 恰好落在 want 列出的位置上（每项是 "文件 位置"、规则），每个位置至少报了那条规则。
func wantRules(t *testing.T, probs []Problem, want [][2]string) {
	t.Helper()
	got := map[string][]string{}
	for _, p := range probs {
		key := p.File
		if p.At != "" {
			key += " " + p.At
		}
		got[key] = append(got[key], p.Rule)
	}
	wanted := map[string]bool{}
	for _, w := range want {
		wanted[w[0]] = true
		found := false
		for _, r := range got[w[0]] {
			found = found || r == w[1]
		}
		if !found {
			t.Errorf("%s 应当报 %s，实际 %v", w[0], w[1], got[w[0]])
		}
	}
	for _, key := range sortedKeys(got) {
		if !wanted[key] {
			t.Errorf("不应当报 %s：%v", key, got[key])
		}
	}
}

func noProblems(t *testing.T, what string, probs []Problem) {
	t.Helper()
	for _, p := range probs {
		t.Errorf("%s：%s", what, p)
	}
}

func TestMeta(t *testing.T) {
	t.Run("C01-T1 元校验", func(t *testing.T) {
		noProblems(t, "仓库的 schema/", repoSet(t).CheckMeta())

		root := writeTree(t, [][2]string{
			{"schema/a.schema.json", schemaDoc("a.schema.json", `"$ref": "b.schema.json#/$defs/X"`)},
			{"schema/b.schema.json", schemaDoc("b.schema.json", `"$defs": {"X": {"type": "string"}}`)},
			{"schema/bad.schema.json", schemaDoc("bad.schema.json", `"type": 5`)},
			{"schema/remote.schema.json", schemaDoc("remote.schema.json", `"$ref": "https://example.com/x.schema.json"`)},
			{"schema/notjson.schema.json", `{"$schema": `},
		})
		s, probs := Load(root)
		wantRules(t, append(probs, s.CheckMeta()...), [][2]string{
			{"schema/bad.schema.json", "C.01-R1"},     // 元校验：type 不能是数字
			{"schema/remote.schema.json", "C.01-R1"},  // 只从本地解析，不联网
			{"schema/notjson.schema.json", "C.01-R1"}, // 不是 JSON
		})
		if err := s.Validate("a.schema.json", parse(t, `"x"`)); err != nil {
			t.Fatalf("跨文件引用应当能解析：%v", err)
		}
		if err := s.Validate("a.schema.json", parse(t, `1`)); err == nil {
			t.Fatal("跨文件引用的约束应当生效")
		}
	})
}

func TestStyle(t *testing.T) {
	t.Run("C01-T2 写法", func(t *testing.T) {
		noProblems(t, "仓库的 schema/", repoSet(t).CheckStyle())

		head := `"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "https://anydoor.local/schema/`
		root := writeTree(t, [][2]string{
			{"schema/common.schema.json", schemaDoc("common.schema.json", `"$defs": {"Ulid": {"type": "string"}}`)},
			{"schema/good.schema.json", schemaDoc("good.schema.json", `"$defs": {"Graph": {"$ref": "common.schema.json#/$defs/Ulid"}}`)},
			{"schema/Bad.schema.json", schemaDoc("Bad.schema.json", "")},
			{"schema/noschema.schema.json", `{"$id": "https://anydoor.local/schema/noschema.schema.json", "x-version": "1.0"}`},
			{"schema/wrongid.schema.json", schemaDoc("other.schema.json", "")},
			{"schema/nover.schema.json", `{` + head + `nover.schema.json"}`},
			{"schema/badver.schema.json", `{` + head + `badver.schema.json", "x-version": "1"}`},
			{"schema/dup.schema.json", schemaDoc("dup.schema.json", `"$defs": {"Ulid": {"type": "string"}}`)},
		})
		s, probs := Load(root)
		wantRules(t, append(probs, s.CheckStyle()...), [][2]string{
			{"schema/Bad.schema.json", "C.01-R1"},
			{"schema/noschema.schema.json", "C.01-R1"},
			{"schema/wrongid.schema.json", "C.01-R2"},
			{"schema/nover.schema.json", "C.01-R21"},
			{"schema/badver.schema.json", "C.01-R21"},
			{"schema/dup.schema.json", "C.01-R3"},
		})
	})
}

func TestNames(t *testing.T) {
	t.Run("C01-T3 命名", func(t *testing.T) {
		noProblems(t, "仓库的 schema/", repoSet(t).CheckNames())

		good := `"type": "object", "additionalProperties": false, "required": ["must"], "properties": {
			"updated_at": {"$ref": "common.schema.json#/$defs/Timestamp"},
			"ts": {"$ref": "common.schema.json#/$defs/Timestamp"},
			"timeout_ms": {"type": "integer"},
			"cost_micros": {"type": "integer"},
			"in_tokens": {"type": "integer"},
			"hit_ratio": {"type": "number", "minimum": 0, "maximum": 1},
			"enabled": {"type": "boolean", "default": true},
			"must": {"type": "boolean"},
			"body_ref": {"$ref": "common.schema.json#/$defs/WsRef"},
			"nested": {"type": "array", "items": {"type": "object", "additionalProperties": false, "properties": {"v2_name": {"type": "string"}}}}
		}`
		bad := `"$defs": {"X": {"type": "object", "additionalProperties": false, "properties": {
			"Bad_Name": {"type": "string"},
			"created": {"$ref": "common.schema.json#/$defs/Timestamp"},
			"start_at": {"type": "string"},
			"wait_ms": {"type": "number"},
			"cost_micros": {"type": "string"},
			"max_tokens": {"type": "number"},
			"hit_ratio": {"type": "number"},
			"enabled": {"type": "boolean"},
			"no_cache": {"type": "boolean", "default": false},
			"disabled": {"type": "boolean", "default": false},
			"body_ref": {"type": "string"},
			"inner": {"type": "array", "items": {"type": "object", "additionalProperties": false, "properties": {"Deep": {"type": "string"}}}}
		}}}`
		root := writeTree(t, [][2]string{
			{"schema/good.schema.json", schemaDoc("good.schema.json", good)},
			{"schema/bad.schema.json", schemaDoc("bad.schema.json", bad)},
		})
		s, probs := Load(root)
		noProblems(t, "读取", probs)
		var want [][2]string
		for _, w := range [][2]string{
			{"Bad_Name", "C.01-R4"},
			{"created", "C.01-R7"},
			{"start_at", "C.01-R7"},
			{"wait_ms", "C.01-R8"},
			{"cost_micros", "C.01-R9"},
			{"max_tokens", "C.01-R10"},
			{"hit_ratio", "C.01-R11"},
			{"enabled", "C.01-R12"},
			{"no_cache", "C.01-R12"},
			{"disabled", "C.01-R12"},
			{"body_ref", "C.01-R13"},
			{"inner/items/properties/Deep", "C.01-R4"},
		} {
			want = append(want, [2]string{"schema/bad.schema.json /$defs/X/properties/" + w[0], w[1]})
		}
		wantRules(t, s.CheckNames(), want)
	})
}

func TestClosed(t *testing.T) {
	t.Run("C01-T4 封闭对象", func(t *testing.T) {
		noProblems(t, "仓库的 schema/", repoSet(t).CheckClosed())

		root := writeTree(t, [][2]string{
			{"schema/x.schema.json", schemaDoc("x.schema.json", `"$defs": {
				"Closed": {"type": "object", "additionalProperties": false},
				"Map": {"type": "object", "additionalProperties": {"type": "string"}},
				"Open": {"type": "object", "x-open": true},
				"Loose": {"type": "object", "properties": {"a": {"type": "string"}}},
				"Both": {"type": "object", "x-open": true, "additionalProperties": false},
				"Wrong": {"type": "object", "x-open": "yes", "additionalProperties": false},
				"Nested": {"type": "array", "items": {"type": ["object", "null"]}},
				"Partial": {"if": {"properties": {"kind": {"const": "a"}}}, "then": {"required": ["a"]}}
			}`)},
		})
		s, probs := Load(root)
		noProblems(t, "读取", probs)
		wantRules(t, s.CheckClosed(), [][2]string{
			{"schema/x.schema.json /$defs/Loose", "C.01-R19"},
			{"schema/x.schema.json /$defs/Both", "C.01-R19"},
			{"schema/x.schema.json /$defs/Wrong", "C.01-R19"},
			{"schema/x.schema.json /$defs/Nested/items", "C.01-R19"},
		})
	})
}

func TestExamples(t *testing.T) {
	t.Run("C01-T5 示例清单", func(t *testing.T) {
		noProblems(t, "仓库的 schema/examples/", repoSet(t).CheckExamples())

		ex := "schema/examples/"
		root := writeTree(t, [][2]string{
			{"schema/t.schema.json", schemaDoc("t.schema.json", `"type": "object", "additionalProperties": false, "required": ["id"],
				"properties": {"id": {"type": "string", "pattern": "^[a-z]+$"}, "n": {"type": "integer"}}`)},
			{ex + "ok.json", `{"id": "a"}`},
			{ex + "should_pass.json", `{"id": "A"}`},
			{ex + "invalid/passes.json", `{"id": "a"}`},
			{ex + "invalid/elsewhere.json", `{"id": "A", "n": "x"}`},
			{ex + "invalid/fragment.json", `{"id": "A"}`},
			{ex + "semantic/nodiag.json", `{"id": "a"}`},
			{ex + "semantic/v05.json", `{"id": "a"}`},
			{ex + "semantic/baddiag.json", `{"id": "a"}`},
			{ex + "nodiag_here.json", `{"id": "a"}`},
			{ex + "novalid.json", `{"id": "a"}`},
			{ex + "unlisted.json", `{"id": "a"}`},
			{ex + "x_vectors.json", `[]`},
			{"schema/outside.json", `{"id": "a"}`}, // 在 schema/examples/ 外面，清单不能引用它
			{ex + "manifest.yaml", `
- {file: ok.json, schema: t.schema.json, valid: true}
- {file: should_pass.json, schema: t.schema.json, valid: true}
- {file: invalid/passes.json, schema: t.schema.json, valid: false, error_at: /id}
- {file: invalid/elsewhere.json, schema: t.schema.json, valid: false, error_at: /id}
- {file: invalid/fragment.json, schema: "t.schema.json#/properties/id", valid: false, error_at: ""}
- {file: semantic/nodiag.json, schema: t.schema.json, valid: true}
- {file: semantic/v05.json, schema: t.schema.json, valid: true, diag: V05}
- {file: semantic/baddiag.json, schema: t.schema.json, valid: true, diag: X1}
- {file: nodiag_here.json, schema: t.schema.json, valid: true, diag: V05}
- {file: novalid.json, schema: t.schema.json}
- {file: missing.json, schema: t.schema.json, valid: true}
- {file: ok.json, schema: t.schema.json, valid: true}
- {file: ..\outside.json, schema: t.schema.json, valid: true}
- {file: ../outside.json, schema: t.schema.json, valid: true}
- {file: /abs.json, schema: t.schema.json, valid: true}
- {file: "C:/x.json", schema: t.schema.json, valid: true}
- {file: "invalid\\passes.json", schema: t.schema.json, valid: false, error_at: /id}
`},
		})
		s, probs := Load(root)
		noProblems(t, "读取", probs)
		var got []string
		for _, p := range s.CheckExamples() {
			got = append(got, p.File+"："+p.Msg)
		}
		all := strings.Join(got, "\n")
		m := ex + "manifest.yaml："
		for _, want := range []string{
			ex + "should_pass.json：应当通过 t.schema.json，实际失败",
			ex + "invalid/passes.json：应当失败，实际通过了 t.schema.json",
			ex + `invalid/elsewhere.json：报错位置应当都在 "/id" 或它下面，实际还有 "/n"`,
			m + "semantic/nodiag.json：",
			m + "semantic/baddiag.json：",
			m + "nodiag_here.json：写了 diag 的示例放在 semantic/ 下",
			m + "novalid.json：要写 valid",
			m + "missing.json：读不了",
			m + "ok.json：重复登记",
			ex + "unlisted.json：没有登记在 manifest.yaml 里",
			m + `..\outside.json：file 要写相对 schema/examples/ 的 .json 路径`, // Windows 上反斜杠也是分隔符
			m + "../outside.json：file 要写相对 schema/examples/ 的 .json 路径",
			m + "/abs.json：file 要写相对 schema/examples/ 的 .json 路径",
			m + "C:/x.json：file 要写相对 schema/examples/ 的 .json 路径",
			m + `invalid\passes.json：file 要写相对 schema/examples/ 的 .json 路径`,
		} {
			if !strings.Contains(all, want) {
				t.Errorf("应当报：%s\n实际：\n%s", want, all)
			}
		}
		for _, quiet := range []string{ex + "ok.json：", "v05.json", "fragment.json", "x_vectors.json", "schema/outside.json"} {
			if strings.Contains(all, quiet) {
				t.Errorf("不应当报 %s：\n%s", quiet, all)
			}
		}
		if len(got) != 15 {
			t.Errorf("应当恰好 15 个问题，实际 %d：\n%s", len(got), all)
		}
	})
}

func TestRelExample(t *testing.T) {
	t.Run("C01-T5/清单路径", func(t *testing.T) {
		for _, f := range []string{"a.json", "invalid/a.json", "semantic/x/y.json", "a..b.json"} {
			if !relExample(f) {
				t.Errorf("%q 应当接受", f)
			}
		}
		for _, f := range []string{"", "a.yaml", "../a.json", "..", `..\a.json`, `invalid\a.json`, "/a.json", "C:/a.json", "c:a.json", "./a.json", "a//b.json", "a/../b.json"} {
			if relExample(f) {
				t.Errorf("%q 应当拒绝", f)
			}
		}
	})
}

func TestCommon(t *testing.T) {
	t.Run("C01-T6 公共定义", func(t *testing.T) {
		s := repoSet(t)
		const ulid = `"01JDN0000000000000000000A1"`
		hex := strings.Repeat("0a", 32)
		for _, c := range []struct {
			def string
			ok  []string
			bad [][2]string // 值、期望的报错位置
		}{
			{"Ulid", []string{ulid, `"7ZZZZZZZZZZZZZZZZZZZZZZZZZ"`}, [][2]string{
				{`"01jdn0000000000000000000a1"`, ""}, {`"01JDN0000000000000000000A"`, ""},
				{`"01JDN000000000000000000IL1"`, ""}, {`"01JDN000000000000000000OU1"`, ""}, {`1`, ""},
			}},
			{"Timestamp", []string{`"2026-11-22T12:01:03.120Z"`}, [][2]string{
				{`"2026-11-22T12:01:03.120+08:00"`, ""}, {`"2026-11-22"`, ""}, {`"2026-13-22T12:01:03.120Z"`, ""},
			}},
			{"WsRef", []string{`"ws://sha256:` + hex + `"`}, [][2]string{
				{`"ws://sha256:` + strings.ToUpper(hex) + `"`, ""}, {`"ws://sha256:abc"`, ""}, {`"sha256:` + hex + `"`, ""},
			}},
			{"Actor", []string{
				`{"kind": "user", "id": "local"}`,
				`{"kind": "model", "id": "large-model", "instance_id": "chat:01JD", "tags": ["编排者"]}`,
				`{"kind": "runtime", "id": "sched"}`,
				`{"kind": "external", "id": "anydoorctl"}`,
			}, [][2]string{
				{`{"kind": "system", "id": "x"}`, "/kind"},
				{`{"kind": "user"}`, ""},
				{`{"kind": "user", "id": "local", "role": "admin"}`, ""},
				{`{"kind": "user", "id": "` + strings.Repeat("x", 201) + `"}`, "/id"},
				{`{"kind": "user", "id": "local", "tags": ["a", "a"]}`, "/tags"},
			}},
			{"MandateMount", []string{
				`{"anchor": "graph", "mandate_id": ` + ulid + `}`,
				`{"anchor": "session", "mandate_id": ` + ulid + `, "priority": 2}`,
				`{"anchor": "node:01JDN0000000000000000000A1", "mandate_id": ` + ulid + `}`,
			}, [][2]string{
				{`{"anchor": "path:根图", "mandate_id": ` + ulid + `}`, "/anchor"},
				{`{"anchor": "node:abc", "mandate_id": ` + ulid + `}`, "/anchor"},
				{`{"anchor": "graph"}`, ""},
				{`{"anchor": "graph", "mandate_id": ` + ulid + `, "x": 1}`, ""},
				{`{"anchor": "graph", "mandate_id": ` + ulid + `, "priority": 1.5}`, "/priority"},
			}},
		} {
			ref := "common.schema.json#/$defs/" + c.def
			for _, v := range c.ok {
				mustValid(t, s, ref, v)
			}
			for _, v := range c.bad {
				mustInvalid(t, s, ref, v[0], v[1])
			}
		}
		if e := manifestEntry(t, s, "invalid/actor_system_kind.json"); e.Schema != "common.schema.json#/$defs/Actor" || e.ErrorAt == nil || *e.ErrorAt != "/kind" {
			t.Errorf("清单里 actor_system_kind.json 的登记：%+v", e)
		}
		if e := manifestEntry(t, s, "invalid/mandate_anchor_path.json"); e.Schema != "common.schema.json#/$defs/MandateMount" || e.ErrorAt == nil || *e.ErrorAt != "/anchor" {
			t.Errorf("清单里 mandate_anchor_path.json 的登记：%+v", e)
		}
		var raw []map[string]any
		readYAML(t, "schema/examples/manifest.yaml", &raw)
		for i, e := range raw {
			for _, k := range sortedKeys(e) {
				if k != "file" && k != "schema" && k != "valid" && k != "error_at" && k != "diag" {
					t.Errorf("清单第 %d 项有不认识的键 %s", i, k)
				}
			}
		}
	})
}

func TestCommand(t *testing.T) {
	t.Run("C01-T7 命令", func(t *testing.T) {
		var out bytes.Buffer
		if code := run([]string{repoRoot}, &out); code != 0 || !strings.Contains(out.String(), "schema 检查通过") {
			t.Fatalf("仓库的 schema/ 应当通过（退出 %d）：\n%s", code, out.String())
		}
		root := writeTree(t, [][2]string{{"schema/Bad.schema.json", `{"type": "object"}`}})
		out.Reset()
		code := run([]string{root}, &out)
		s := out.String()
		if code != 1 {
			t.Fatalf("有问题时退出 1，实际 %d：\n%s", code, s)
		}
		for _, want := range []string{"schema/Bad.schema.json：C.01-R1：", "：C.01-R2：", "：C.01-R21：", "schema/Bad.schema.json：C.01-R19：", "== schema 检查没有通过："} {
			if !strings.Contains(s, want) {
				t.Errorf("输出里没有 %q：\n%s", want, s)
			}
		}
		out.Reset()
		if code := run([]string{t.TempDir()}, &out); code != 2 {
			t.Errorf("没有 schema/ 目录时退出 2，实际 %d", code)
		}
		if code := run([]string{"a", "b"}, &out); code != 2 {
			t.Errorf("参数太多时退出 2，实际 %d", code)
		}
	})
}
