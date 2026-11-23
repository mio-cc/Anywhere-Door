package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// atomicNodes 是 13 个原子节点（C.03.01 §6）。
var atomicNodes = []string{"资源", "Agent", "上下文装配", "工具", "扇出", "汇合", "聚合", "分支", "循环", "账本", "人工", "子图端口", "子图引用"}

const nodedefDir = "schema/builtin/nodedefs"

// portSig 是端口签名里测试要核对的部分。
type portSig struct {
	Type     string
	Required bool
	Multi    bool
	Dynamic  bool
}

// nodeDoc 是定义文件里测试要用到的部分；完整结构由 nodedef.schema.json 校验。
type nodeDoc struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Category string `json:"category"`
	Ports    struct {
		Inputs  map[string]portDoc `json:"inputs"`
		Outputs map[string]portDoc `json:"outputs"`
	} `json:"ports"`
	Params       map[string]map[string]any `json:"params"`
	TypeVars     map[string]string         `json:"type_vars"`
	Container    string                    `json:"container"`
	DynamicPorts *struct {
		FromParam string `json:"from_param"`
	} `json:"dynamic_ports"`
}

type portDoc struct {
	Type     string `json:"type"`
	Category string `json:"category"`
	Required bool   `json:"required"`
	Multi    bool   `json:"multi"`
	Dynamic  bool   `json:"dynamic"`
}

func (p portDoc) sig() portSig {
	return portSig{Type: p.Type, Required: p.Required, Multi: p.Multi, Dynamic: p.Dynamic}
}

// loadNodeDef 读一个定义文件，返回给校验器的值和解析后的结构。
func loadNodeDef(t *testing.T, name string) (any, nodeDoc) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(nodedefDir), name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	j, err := yaml.YAMLToJSON(b)
	if err != nil {
		t.Fatalf("%s.yaml 不是合法的 YAML：%v", name, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(j))
	if err != nil {
		t.Fatal(err)
	}
	var doc nodeDoc
	if err := json.Unmarshal(j, &doc); err != nil {
		t.Fatalf("%s.yaml：%v", name, err)
	}
	return inst, doc
}

func TestNodeDefs(t *testing.T) {
	t.Run("C0301-T1 定义文件校验", func(t *testing.T) {
		s := repoSet(t)
		entries, err := os.ReadDir(filepath.Join(repoRoot, filepath.FromSlash(nodedefDir)))
		if err != nil {
			t.Fatal(err)
		}
		var files []string
		for _, e := range entries {
			files = append(files, e.Name())
		}
		want := make([]string, len(atomicNodes))
		for i, n := range atomicNodes {
			want[i] = n + ".yaml"
		}
		sort.Strings(files)
		sort.Strings(want)
		if !reflect.DeepEqual(files, want) {
			t.Errorf("%s 下的文件：%v，应恰好是 13 个原子节点：%v", nodedefDir, files, want)
		}
		for _, n := range atomicNodes {
			inst, doc := loadNodeDef(t, n)
			if err := s.Validate("nodedef.schema.json", inst); err != nil {
				t.Errorf("%s.yaml 没有通过 nodedef.schema.json：%v", n, err)
			}
			if doc.Name != n {
				t.Errorf("%s.yaml 的 name 是 %q，应与文件名相同", n, doc.Name)
			}
			if doc.Kind != "atomic" {
				t.Errorf("%s.yaml 的 kind 是 %q，应为 atomic（C.03.01-R1）", n, doc.Kind)
			}
		}
		// 根结构的反例（测试里内联）
		const ok = `{"name": "x", "version": "1.0", "kind": "extension", "category": "扩展", "impl": {"type": "builtin"}, "ports": {"inputs": {}, "outputs": {}}, "params": {}`
		mustValid(t, s, "nodedef.schema.json", ok+`}`)
		mustInvalid(t, s, "nodedef.schema.json", ok+`, "extra": 1}`, "")
		mustInvalid(t, s, "nodedef.schema.json", strings.Replace(ok, `"1.0"`, `"1"`, 1)+`}`, "/version")
		mustInvalid(t, s, "nodedef.schema.json", strings.Replace(ok, `"extension"`, `"plugin"`, 1)+`}`, "/kind")
		mustInvalid(t, s, "nodedef.schema.json", strings.Replace(ok, `"扩展"`, `"其他"`, 1)+`}`, "/category")
		// 键名不合法（propertyNames）：校验器把这类错误记在根位置，不在对象下面，所以位置写 ""，
		// 只断言被拒绝；它是因为键名被拒、不是别的原因，由同一段 ok 去掉这个键后能通过保证
		mustInvalid(t, s, "nodedef.schema.json", strings.Replace(ok, `"inputs": {}`, `"inputs": {"In": {"type": "任意"}}`, 1)+`}`, "")
		mustInvalid(t, s, "nodedef.schema.json", strings.Replace(ok, `"inputs": {}`, `"inputs": {"in": {"type": "任意", "multi": true, "dynamic": true}}`, 1)+`}`, "/ports/inputs/in")
		mustInvalid(t, s, "nodedef.schema.json", ok+`, "type_vars": {"t": "input:in"}}`, "")
		mustValid(t, s, "nodedef.schema.json", ok+`, "type_vars": {"T": "input:in"}}`)
		mustInvalid(t, s, "nodedef.schema.json", ok+`, "dynamic_ports": {"from_param": "value", "max": 33}}`, "/dynamic_ports/max")
		mustInvalid(t, s, "nodedef.schema.json", strings.Replace(ok, `{"type": "builtin"}`, `{"type": "script"}`, 1)+`}`, "/impl/type")
	})
	t.Run("C0301-T2 端口签名", func(t *testing.T) {
		// C.03.01 §6 的表（多 = multi，必 = required，动态 = dynamic）
		type sig = portSig
		want := map[string][2]map[string]sig{
			"资源":    {{}, {"ref": {Type: "资源引用@1"}}},
			"Agent": {{"context": {Type: "上下文包@1", Required: true}, "model": {Type: "资源引用@1", Required: true}, "prompt": {Type: "资源引用@1"}, "role": {Type: "资源引用@1"}, "tools": {Type: "工具端点@1", Multi: true}}, {"result": {Type: "$T"}}},
			"上下文装配": {{"in": {Type: "任意", Multi: true}, "model": {Type: "资源引用@1"}}, {"context": {Type: "上下文包@1"}, "dropped": {Type: "列表[任意]"}}},
			"工具":    {{"request": {Type: "任意"}}, {"endpoint": {Type: "工具端点@1"}, "result": {Type: "$T"}}},
			"扇出":    {{"items": {Type: "列表[$T]"}, "shared": {Type: "任意", Multi: true}}, {"results": {Type: "列表[$U]"}}},
			"汇合":    {{"in": {Type: "任意", Dynamic: true}}, {"collected": {Type: "列表[任意]"}}},
			"聚合":    {{"items": {Type: "列表[$T]", Required: true}}, {"result": {Type: "$U"}}},
			"分支":    {{"value": {Type: "$T", Required: true}}, {"yes": {Type: "$T"}, "no": {Type: "$T"}}},
			"循环":    {{"init": {Type: "$T"}}, {"final": {Type: "$T"}, "summary": {Type: "简报@1"}}},
			"账本":    {{"value": {Type: "任意"}}, {"value": {Type: "状态@1"}}},
			"人工":    {{"request": {Type: "$T", Required: true}}, {"approved": {Type: "$T"}, "rejected": {Type: "$T"}}},
			"子图端口":  {{"in": {Type: "$T"}}, {"out": {Type: "$T"}}},
			"子图引用":  {{}, {}},
		}
		ctrlOut := map[string]bool{"分支": true, "人工": true}
		for _, n := range atomicNodes {
			_, doc := loadNodeDef(t, n)
			for i, ports := range []map[string]portDoc{doc.Ports.Inputs, doc.Ports.Outputs} {
				side := []string{"输入", "输出"}[i]
				got := map[string]sig{}
				for name, p := range ports {
					got[name] = p.sig()
					wantCat := ""
					if i == 1 && ctrlOut[n] {
						wantCat = "ctrl" // 分支、人工的出口显示为"判定"（§6 说明、K15）
					}
					if p.Category != wantCat {
						t.Errorf("%s 的%s端口 %s：category 是 %q，应为 %q", n, side, name, p.Category, wantCat)
					}
				}
				if w := want[n][i]; !reflect.DeepEqual(got, w) && (len(got) != 0 || len(w) != 0) {
					t.Errorf("%s 的%s端口：%+v\n应为 §6：%+v", n, side, got, w)
				}
			}
		}
		_, agent := loadNodeDef(t, "Agent")
		if c := agent.Ports.Inputs["context"]; !c.Required {
			t.Error("Agent 的 context 端口应为必填")
		}
	})
	t.Run("C0301-T3 定义内部一致", func(t *testing.T) {
		var cr []struct {
			Type     string `yaml:"type"`
			Children string `yaml:"children"`
			Ref      string `yaml:"ref"`
		}
		readYAML(t, "schema/builtin/container_rules.yaml", &cr)
		container := map[string]string{}
		for _, r := range cr {
			switch {
			case r.Children == "required" && r.Ref == "forbidden":
				container[r.Type] = "required"
			case r.Children == "one_of" && r.Ref == "one_of":
				container[r.Type] = "one_of"
			default:
				t.Errorf("container_rules.yaml 里 %s 的写法本测试不认识：%+v", r.Type, r)
			}
		}
		typeVars := map[string]map[string]string{ // §6 的"类型变量"列
			"Agent": {"T": "param:output_schema"},
			"工具":    {"T": "param:result_type"},
			"扇出":    {"T": "input:items.elem", "U": "body:exit"},
			"聚合":    {"T": "input:items.elem", "U": "op:op(T)"},
			"分支":    {"T": "input:value"},
			"循环":    {"T": "input:init"},
			"人工":    {"T": "input:request"},
			"子图端口":  {"T": "input:in"},
		}
		for _, n := range atomicNodes {
			_, doc := loadNodeDef(t, n)
			for p := range doc.Params {
				if _, ok := doc.Ports.Inputs[p]; ok {
					t.Errorf("%s：参数 %s 和输入端口重名（C.03.01-R3）", n, p)
				}
				if _, ok := doc.Ports.Outputs[p]; ok {
					t.Errorf("%s：参数 %s 和输出端口重名（C.03.01-R3）", n, p)
				}
			}
			for i, ports := range []map[string]portDoc{doc.Ports.Inputs, doc.Ports.Outputs} {
				if len(ports) > 8 {
					t.Errorf("%s 的%s端口有 %d 个，最多 8 个（C.03.01-R5）", n, []string{"输入", "输出"}[i], len(ports))
				}
			}
			wantC := container[n]
			if wantC == "" {
				wantC = "none"
			}
			gotC := doc.Container
			if gotC == "" {
				gotC = "none"
			}
			if gotC != wantC {
				t.Errorf("%s 的 container 是 %s，container_rules.yaml 里是 %s（C.02.04 §5）", n, gotC, wantC)
			}
			tv := doc.TypeVars
			if tv == nil {
				tv = map[string]string{}
			}
			w := typeVars[n]
			if w == nil {
				w = map[string]string{}
			}
			if !reflect.DeepEqual(tv, w) {
				t.Errorf("%s 的 type_vars：%v，应为 §6：%v", n, tv, w)
			}
			common := 0
			for p, def := range doc.Params {
				ui, _ := def["ui"].(map[string]any)
				if ui["group"] == "常用" {
					common++
				}
				if b, ok := ui["bind"].(string); ok {
					if _, ok := doc.Ports.Inputs[b]; !ok {
						t.Errorf("%s 的参数 %s 绑定的端口 %s 不存在（C.03.05-R2）", n, p, b)
					}
				}
			}
			if common > 4 {
				t.Errorf("%s 有 %d 个常用参数，最多 4 个（C.03.05-R1）", n, common)
			}
		}
	})
	t.Run("C0301-T4 内核锁参数", func(t *testing.T) {
		// C.03.03 §5 的表
		want := map[string]map[string]any{
			"Agent": {"context_src": "上下文装配"},
			"上下文装配": {"honor_inherit": true},
			"工具":    {"keep_pointer": true},
			"聚合":    {"deterministic": true},
			"分支":    {"pure": true},
		}
		for _, n := range atomicNodes {
			_, doc := loadNodeDef(t, n)
			got := map[string]any{}
			for p, def := range doc.Params {
				if def["lock"] == "kernel" {
					got[p] = def["default"]
				}
			}
			w := want[n]
			if w == nil {
				w = map[string]any{}
			}
			if !reflect.DeepEqual(got, w) {
				t.Errorf("%s 的 kernel 锁参数及默认值：%v，应为 C.03.03 §5：%v", n, got, w)
			}
		}
	})
	t.Run("C0301-T5 动态端口", func(t *testing.T) {
		want := map[string]string{"资源": "template", "工具": "args"}
		for _, n := range atomicNodes {
			_, doc := loadNodeDef(t, n)
			from := ""
			if doc.DynamicPorts != nil {
				from = doc.DynamicPorts.FromParam
			}
			if from != want[n] {
				t.Errorf("%s 的 dynamic_ports.from_param 是 %q，应为 %q", n, from, want[n])
				continue
			}
			if from == "" {
				continue
			}
			def, ok := doc.Params[from]
			if !ok {
				t.Errorf("%s：dynamic_ports 指向的参数 %s 不存在", n, from)
				continue
			}
			if n == "工具" && def["schema_from"] != "param:tool" {
				t.Errorf("工具的 args 应写 schema_from: param:tool（C.03.02 §5），实际 %v", def["schema_from"])
			}
		}
	})
}
