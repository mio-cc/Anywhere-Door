package main

import (
	"bytes"
	"flag"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot 是仓库根目录（测试在 tools/gen 下运行）。
const repoRoot = "../.."

var update = flag.Bool("update", false, "重写 testdata/golden 下的黄金文件（C14-T3）")

func repoModel(t *testing.T) *model {
	t.Helper()
	set, err := loadSchemas(filepath.Join(repoRoot, "schema"))
	if err != nil {
		t.Fatal(err)
	}
	al, err := readAliases(filepath.Join(repoRoot, "schema", "builtin", "aliases.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := buildModel(set, al.Roots)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestGenerate(t *testing.T) {
	t.Run("C14-T1 确定性", func(t *testing.T) {
		a, err := Generate(repoRoot)
		if err != nil {
			t.Fatal(err)
		}
		b, err := Generate(repoRoot)
		if err != nil {
			t.Fatal(err)
		}
		if len(a) != len(b) {
			t.Fatalf("两次生成的文件数不同：%d、%d", len(a), len(b))
		}
		for i := range a {
			if a[i].Path != b[i].Path || !bytes.Equal(a[i].Data, b[i].Data) {
				t.Errorf("%s：两次生成的结果不同（G.16-R2）", a[i].Path)
			}
			committed, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(a[i].Path)))
			if err != nil {
				t.Errorf("%s：仓库里没有这个生成物，先运行 go run ./tools/gen：%v", a[i].Path, err)
				continue
			}
			if !bytes.Equal(committed, a[i].Data) {
				t.Errorf("%s：和仓库里提交的不同，运行 go run ./tools/gen 后提交（G.16-R3）", a[i].Path)
			}
		}
	})
	t.Run("C14-T3 黄金文件", func(t *testing.T) {
		outs, _, err := generateCore(filepath.Join("testdata", "schema"), filepath.Join("testdata", "aliases.yaml"), filepath.Join("testdata", "sse_control.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range outs {
			p := filepath.Join("testdata", "golden", path.Base(o.Path))
			if *update {
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, o.Data, 0o644); err != nil {
					t.Fatal(err)
				}
				continue
			}
			want, err := os.ReadFile(p)
			if err != nil {
				t.Errorf("%s：没有黄金文件，用 go test ./tools/gen -run TestGenerate -update 生成后检查内容再提交：%v", p, err)
				continue
			}
			if !bytes.Equal(want, o.Data) {
				t.Errorf("%s：生成结果和黄金文件不同（G.16-R5）；确认改动是有意的，再用 -update 重写", p)
			}
		}
	})
}

func modelFrom(t *testing.T, files, roots map[string]string) (*model, error) {
	t.Helper()
	set, err := newSchemaSet(files)
	if err != nil {
		t.Fatal(err)
	}
	return buildModel(set, roots)
}

func TestNames(t *testing.T) {
	t.Run("C14-T4 命名与拒绝", func(t *testing.T) {
		for in, want := range map[string]string{
			"node_ids": "NodeIDs", "instance_id": "InstanceID", "l3_skip_human": "L3SkipHuman",
			"ui": "UI", "content_ref": "ContentRef", "llm_calls": "LLMCalls", "url": "URL",
		} {
			if got := goName(in); got != want {
				t.Errorf("goName(%q) = %q，应为 %q", in, got, want)
			}
		}
		for in, want := range map[string][2]string{ // 值 → {Go, TS}
			"tool.L0": {"ToolL0", "ToolL0"}, "V09": {"V09", "V09"}, "llm.call": {"LLMCall", "LlmCall"},
			"op.add_node": {"OpAddNode", "OpAddNode"}, "graph_conflict": {"GraphConflict", "GraphConflict"},
		} {
			g, err1 := constName(in, true)
			ts, err2 := constName(in, false)
			if err1 != nil || err2 != nil || g != want[0] || ts != want[1] {
				t.Errorf("constName(%q) = %q / %q（%v %v），应为 %q / %q", in, g, ts, err1, err2, want[0], want[1])
			}
		}
		if _, err := constName("资源", true); err == nil {
			t.Error("中文取值应当换不成常量名（要用 aliases.yaml）")
		}
		for _, bad := range []string{"x_2", "a__b", "a_", "Abc", "2a"} {
			if checkFieldName(bad) == nil {
				t.Errorf("字段名 %q 应当被拒绝（C.01-R4、C.14-R9）", bad)
			}
		}
		const head = `"$schema": "https://json-schema.org/draft/2020-12/schema"`
		for _, c := range []struct {
			name  string
			files map[string]string
			roots map[string]string
			want  string
		}{
			{"字段名不能往返", map[string]string{"a.schema.json": `{` + head + `, "$defs": {"A": {"type": "object", "additionalProperties": false, "properties": {"x_2": {"type": "string"}}}}}`}, nil, "C.14-R9"},
			{"类型重名", map[string]string{
				"a.schema.json": `{` + head + `, "$defs": {"A": {"type": "string"}}}`,
				"b.schema.json": `{` + head + `, "$defs": {"A": {"type": "integer"}}}`}, nil, "重名"},
			{"根类型名没登记", map[string]string{"a.schema.json": `{` + head + `, "type": "object", "additionalProperties": false, "properties": {"x": {"type": "string"}}}`}, nil, "没有登记"},
			{"内联类型和 $defs 重名", map[string]string{"a.schema.json": `{` + head + `, "$defs": {"AB": {"type": "string"}, "A": {"type": "object", "additionalProperties": false, "properties": {"b": {"type": "object", "additionalProperties": false, "properties": {"c": {"type": "string"}}}}}}}`}, nil, "重名"},
		} {
			_, err := modelFrom(t, c.files, c.roots)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s：应当失败并提到 %q，实际：%v", c.name, c.want, err)
			}
		}
		m, err := modelFrom(t, map[string]string{"a.schema.json": `{` + head + `, "$defs": {"E": {"enum": ["a.b", "a_b"]}}}`}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := collectConsts(m, nil, nil); err == nil || !strings.Contains(err.Error(), "重名") {
			t.Errorf("常量重名应当失败，实际：%v", err)
		}
	})
}
