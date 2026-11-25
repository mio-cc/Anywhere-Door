package main

// B.04.15 接口结构：M1 新加进 api.schema.json 的结构（P.09 K260）。E.07.01 的 6 个结构在 api_test.go，这里只读不改。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

const apiWS = "ws://sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// m1Text 读仓库里的一个文件（相对仓库根目录）；.yaml 转成 JSON。
func m1Text(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(rel, ".yaml") {
		j, err := yaml.YAMLToJSON(b)
		if err != nil {
			t.Fatalf("%s：%v", rel, err)
		}
		return string(j)
	}
	return string(b)
}

// m1Valid 是每个新结构的合法例（B0415-T1）；值里引用的大结构从仓库的示例和内置文件读。
func m1Valid(t *testing.T) map[string]string {
	t.Helper()
	graph := m1Text(t, "schema/examples/graph_minimal.json")
	session := m1Text(t, "schema/examples/session/session.json")
	diag := m1Text(t, "schema/examples/errors/diag_v12_warn.json")
	loc := `{"node_id": "` + apiID1 + `", "path": "根图/写作"}`
	return map[string]string{
		"GraphMetaPage":        `{"items": [` + apiValid["GraphMeta"] + `], "next_cursor": "c2"}`,
		"OpRecordPage":         `{"items": [` + apiValid["OpRecord"] + `]}`,
		"RunMetaPage":          `{"items": [` + apiValid["RunMeta"] + `], "next_cursor": "c2"}`,
		"SessionPage":          `{"items": [` + session + `]}`,
		"GraphCreate":          `{"title": "写作流水线"}`,
		"GraphRead":            `{"graph": ` + graph + `, "registry_hash": "sha256:ab12", "diags": [` + diag + `]}`,
		"GraphMetaUpdate":      `{"base_version": 3, "title": "新标题"}`,
		"GraphValidation":      `{"version": 3, "diags": [` + diag + `]}`,
		"GraphConflictDetails": `{"head": 44, "missed_ops": [` + apiValid["OpRecord"] + `], "conflicts": [{"op_id": "` + apiID2 + `", "object": "node:` + apiID1 + `"}]}`,
		"UndoRequest":          `{"branch": "main"}`,
		"RunStart":             `{"graph_id": "` + apiID1 + `"}`,
		"RunStarted":           `{"run_id": "` + apiID2 + `", "graph_version": 3, "status": "pending"}`,
		"RunView":              `{"meta": ` + apiValid["RunMeta"] + `, "current_path": [` + loc + `], "pending_approvals": 0}`,
		"NodeSnapshot":         `{"seq": 17, "items": [` + apiValid["NodeStateRow"] + `]}`,
		"NodeDetail": `{"exec_key": "x1", "node_id": "` + apiID1 + `", "loc": ` + loc + `, "state": "running", "attempt": 1` +
			`, "inputs": [], "outputs": [], "llm_calls": [], "tool_calls": [], "cost_micros": 0}`,
		"NodeDefItem":       `{"ref": "Agent", "package": "core", "def": ` + m1Text(t, "schema/builtin/nodedefs/Agent.yaml") + `}`,
		"TypeItem":          `{"ref": "任务卡@1.0", "package": "core", "def": ` + m1Text(t, "schema/builtin/types/任务卡.yaml") + `}`,
		"TypeCompatRequest": `{"src": "任务卡@1", "dst": "任意"}`,
		"RecipeItem":        `{"ref": "主脑配方@1", "package": "core", "def": ` + m1Text(t, "schema/builtin/recipes/主脑配方.yaml") + `}`,
		"RolecardItem":      `{"ref": "主脑@1", "package": "core", "def": ` + m1Text(t, "schema/builtin/rolecards/主脑.yaml") + `}`,
		"Effort":            `"high"`,
		"Model": `{"ref": "mistral/small", "provider": "mistral", "model": "small", "window": 128000, "max_output": 4096` +
			`, "caps": {"tools": true, "parallel_tools": true, "vision": false, "native_schema": true, "prompt_cache": false}, "effort": ["low", "high"], "priced": false}`,
		"SessionCreate":      `{"title": "写一首诗"}`,
		"SessionUpdate":      `{"title": "写一首诗"}`,
		"SessionGraphAttach": `{"graph_id": "` + apiID1 + `"}`,
	}
}

// m1Names 是本叶子新加的结构，按名字排序。
var m1Names = []string{
	"Effort", "GraphConflictDetails", "GraphCreate", "GraphMetaPage", "GraphMetaUpdate", "GraphRead", "GraphValidation",
	"Model", "NodeDefItem", "NodeDetail", "NodeSnapshot", "OpRecordPage", "RecipeItem", "RolecardItem", "RunMetaPage",
	"RunStart", "RunStarted", "RunView", "SessionCreate", "SessionGraphAttach", "SessionPage", "SessionUpdate",
	"TypeCompatRequest", "TypeItem", "UndoRequest",
}

// m1Defs 是 schema/<file> 的 $defs 的名字。
func m1Defs(t *testing.T, file string) map[string]json.RawMessage {
	t.Helper()
	var doc struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal([]byte(m1Text(t, "schema/"+file)), &doc); err != nil {
		t.Fatalf("%s：%v", file, err)
	}
	return doc.Defs
}

func TestAPISchemaM1(t *testing.T) {
	ref := func(name string) string { return "api.schema.json#/$defs/" + name }

	t.Run("B0415-T1 合法例", func(t *testing.T) {
		s := repoSet(t)
		valid := m1Valid(t)
		names := make([]string, 0, len(valid))
		for name := range valid {
			names = append(names, name)
		}
		sort.Strings(names)
		if strings.Join(names, ",") != strings.Join(m1Names, ",") {
			t.Fatalf("合法例的结构是 %v，应当是 %v", names, m1Names)
		}
		for _, name := range names {
			mustValid(t, s, ref(name), valid[name])
		}
		for _, v := range []string{`{"items": []}`} {
			for _, p := range []string{"GraphMetaPage", "OpRecordPage", "RunMetaPage", "SessionPage"} {
				mustValid(t, s, ref(p), v)
			}
		}
		mustValid(t, s, ref("GraphCreate"), `{}`)
		mustValid(t, s, ref("SessionCreate"), `{}`)
		mustValid(t, s, ref("RunStart"), `{"graph_id": "`+apiID1+`", "branch": "draft/a", "version": 3, "inputs": {"题目": "写诗", "次数": 2}, "session_id": "`+apiID2+`"}`)
		mustValid(t, s, ref("NodeDetail"), `{"exec_key": "x1", "node_id": "`+apiID1+`", "loc": {"node_id": "`+apiID1+`", "path": "根图/写作"}, "state": "failed", "attempt": 2`+
			`, "inputs": [{"port": "指令", "value_ref": "`+apiWS+`"}], "outputs": [{"port": "结果", "value_ref": "`+apiWS+`", "partial": false}]`+
			`, "ctx_ref": "`+apiWS+`", "prompt_ref": "`+apiWS+`"`+
			`, "llm_calls": [{"pool": "default", "provider": "mistral", "model": "small", "usage": {"in": 10, "out": 20}, "cost_micros": 0, "latency_ms": 120, "req_ref": "`+apiWS+`", "resp_ref": "`+apiWS+`", "strategy": "native_schema"}]`+
			`, "tool_calls": [{"tool": "http", "risk": "read", "idem_key": "k1", "queued_ms": 0, "latency_ms": 5, "req_ref": "`+apiWS+`", "result_ref": "`+apiWS+`", "taint": false, "status": "ok"}]`+
			`, "error": {"class": "transient", "code": "timeout", "message": "超时", "retryable": true}, "cost_micros": 30, "started_at": "`+apiTS+`", "ended_at": "`+apiTS+`"}`)
		mustValid(t, s, ref("TypeItem"), strings.TrimSuffix(valid["TypeItem"], "}")+`, "chain": ["方案@1"]}`)
		mustValid(t, s, ref("Model"), `{"ref": "local/qwen2.5:7b", "provider": "local", "model": "qwen2.5:7b", "window": 32000, "max_output": 4096`+
			`, "caps": {"tools": false, "parallel_tools": false, "vision": false, "native_schema": false, "prompt_cache": false}, "priced": false}`)
		for _, v := range []string{
			`{"pinned": true}`,
			`{"active_graph_id": "` + apiID1 + `"}`,
			`{"orchestrator_model": ""}`,
			`{"orchestrator_effort": {"mistral/small": "high", "local/qwen": "off"}}`,
			`{"last_location": {"graph_id": "` + apiID1 + `", "scope": "root"}}`,
		} {
			mustValid(t, s, ref("SessionUpdate"), v)
		}
	})

	t.Run("B0415-T2 反例", func(t *testing.T) {
		s := repoSet(t)
		valid := m1Valid(t)
		for _, name := range m1Names {
			if name == "Effort" {
				continue
			}
			v := valid[name]
			mustInvalid(t, s, ref(name), strings.TrimSuffix(v, "}")+`, "zzz_extra": 1}`, "")
		}
		bad := func(name, old, repl, at string) {
			t.Helper()
			v := valid[name]
			if !strings.Contains(v, old) {
				t.Fatalf("%s 的合法例里没有 %q", name, old)
			}
			mustInvalid(t, s, ref(name), strings.Replace(v, old, repl, 1), at)
		}
		bad("GraphRead", `, "registry_hash": "sha256:ab12"`, ``, "")
		bad("GraphMetaUpdate", `"base_version": 3`, `"base_version": -1`, "/base_version")
		bad("GraphMetaUpdate", `, "title": "新标题"`, ``, "")
		bad("GraphConflictDetails", `"version": 3, `, ``, "/missed_ops/0")
		bad("GraphConflictDetails", `, "object": "node:`+apiID1+`"`, ``, "/conflicts/0")
		bad("UndoRequest", `"main"`, `"feature/x"`, "/branch")
		bad("RunStart", `"`+apiID1+`"`, `"x"`, "/graph_id")
		mustInvalid(t, s, ref("RunStart"), `{"graph_id": "`+apiID1+`", "branch": "feature/x"}`, "/branch")
		bad("RunStarted", `"pending"`, `"started"`, "/status")
		bad("RunView", `"pending_approvals": 0`, `"pending_approvals": -1`, "/pending_approvals")
		bad("RunView", `, "current_path": [`, `, "path": [`, "")
		bad("NodeSnapshot", `"seq": 17, `, ``, "")
		bad("NodeDetail", `"attempt": 1`, `"attempt": 0`, "/attempt")
		bad("NodeDetail", `"cost_micros": 0}`, `"cost_micros": 0, "ctx_ref": "ws://x"}`, "/ctx_ref")
		bad("NodeDetail", `"outputs": []`, `"outputs": [{"port": "结果", "value_ref": "`+apiWS+`"}]`, "/outputs/0")
		bad("NodeDetail", `, "llm_calls": []`, ``, "")
		bad("NodeDefItem", `"package": "core", `, ``, "")
		bad("TypeItem", `"任务卡@1.0"`, `"方案"`, "/ref")
		bad("TypeCompatRequest", `"dst": "任意"`, `"dst": "a/b"`, "/dst")
		bad("RecipeItem", `"ref": "主脑配方@1"`, `"ref": ""`, "/ref")
		bad("RolecardItem", `"package": "core"`, `"package": ""`, "/package")
		mustInvalid(t, s, ref("Effort"), `"medium-high"`, "")
		bad("Model", `, "prompt_cache": false`, ``, "/caps")
		bad("Model", `"effort": ["low", "high"]`, `"effort": ["low", "low"]`, "/effort")
		bad("Model", `, "priced": false`, ``, "")
		bad("SessionCreate", `"写一首诗"`, `"`+strings.Repeat("长", 81)+`"`, "/title")
		mustInvalid(t, s, ref("SessionUpdate"), `{}`, "")
		mustInvalid(t, s, ref("SessionUpdate"), `{"orchestrator_effort": {"mistral/small": "medium-high"}}`, "/orchestrator_effort")
		mustInvalid(t, s, ref("SessionUpdate"), `{"last_location": {"graph_id": "`+apiID1+`", "scope": "子图"}}`, "/last_location")
		bad("SessionGraphAttach", `"graph_id"`, `"graph"`, "")
		bad("GraphMetaPage", `, "next_cursor": "c2"`, `, "next_cursor": ""`, "/next_cursor")
		bad("SessionPage", `{"items": [`, `{"items": [1, `, "/items/0")
	})

	t.Run("B0415-T3 前端要用的结构名", func(t *testing.T) {
		api := m1Defs(t, "api.schema.json")
		for _, name := range []string{"GraphRead", "GraphConflictDetails", "RunStarted", "RunView", "NodeSnapshot", "NodeDetail", "Model"} {
			if _, ok := api[name]; !ok {
				t.Errorf("api.schema.json 里没有 %s（W.04.04 §3.5）", name)
			}
		}
		op := m1Defs(t, "op.schema.json")
		for _, name := range []string{"OpsApplied", "OpsProposed"} {
			if _, ok := op[name]; !ok {
				t.Errorf("op.schema.json 里没有 %s（W.04.04 §3.5）", name)
			}
		}
	})

	t.Run("B0415-T4 已有结构不变", func(t *testing.T) {
		api := m1Defs(t, "api.schema.json")
		want := map[string][]string{
			"GraphMeta":    {"id", "title", "version", "stats", "created_at", "updated_at"},
			"BranchMeta":   {"name", "base_version", "head", "behind", "ahead", "created_at"},
			"OpRecord":     {"graph_id", "branch", "version", "op", "undone"},
			"RunSteps":     {"done", "total"},
			"RunMeta":      {"id", "kind", "graph_id", "branch", "graph_version", "status", "no", "steps", "usage", "cost_micros", "last_seq", "started_by", "created_at"},
			"NodeStateRow": {"exec_key", "node_id", "loc", "state", "attempt", "last_seq"},
		}
		for name, req := range want {
			var def struct {
				Required []string `json:"required"`
			}
			raw, ok := api[name]
			if !ok {
				t.Errorf("api.schema.json 里没有 %s（E.07.01）", name)
				continue
			}
			if err := json.Unmarshal(raw, &def); err != nil {
				t.Fatal(err)
			}
			if strings.Join(def.Required, ",") != strings.Join(req, ",") {
				t.Errorf("%s 的 required 是 %v，E.07.01 定的是 %v（只增不改，C.01-R17）", name, def.Required, req)
			}
		}
		if len(api) != len(want)+len(m1Names) {
			t.Errorf("api.schema.json 有 %d 个结构，应当是 E.07.01 的 %d 个加本叶子的 %d 个", len(api), len(want), len(m1Names))
		}
		s := repoSet(t)
		for name, v := range apiValid {
			mustValid(t, s, ref(name), v)
		}
	})
}
