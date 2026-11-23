package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type assignType struct {
	Name        string   `json:"name"`
	Major       int      `json:"major"`
	Minor       int      `json:"minor"`
	Extends     string   `json:"extends,omitempty"`
	MigrateFrom []string `json:"migrate_from,omitempty"`
	Producer    string   `json:"producer,omitempty"`
}

type assignResult struct {
	OK      bool   `json:"ok"`
	Expand  bool   `json:"expand,omitempty"`
	Migrate string `json:"migrate,omitempty"`
	Rule    string `json:"rule"`
	Reason  string `json:"reason,omitempty"`
}

type assignVector struct {
	Name         string          `json:"name"`
	Src          string          `json:"src"`
	Dst          string          `json:"dst"`
	SrcNode      string          `json:"src_node,omitempty"`
	DstContainer bool            `json:"dst_container,omitempty"`
	Want         json.RawMessage `json:"want"`
}

type assignFile struct {
	Types   []assignType   `json:"types"`
	Vectors []assignVector `json:"vectors"`
}

func readAssignVectors(t *testing.T) assignFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", "assign_vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f assignFile
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		t.Fatalf("assign_vectors.json：%v", err)
	}
	return f
}

var namedRe = regexp.MustCompile(`^([^@\[\]/\s]{1,30})@(\d+)(?:\.\d+)?$`)

type typeRef struct {
	kind  string // any、list、named
	elem  string // list 的元素
	name  string // named
	major int
}

func parseRef(s string) typeRef {
	switch {
	case s == "任意":
		return typeRef{kind: "any"}
	case strings.HasPrefix(s, "列表[") && strings.HasSuffix(s, "]"):
		return typeRef{kind: "list", elem: strings.TrimSuffix(strings.TrimPrefix(s, "列表["), "]")}
	}
	m := namedRe.FindStringSubmatch(s)
	if m == nil {
		return typeRef{kind: "bad"}
	}
	major, _ := strconv.Atoi(m[2])
	return typeRef{kind: "named", name: m[1], major: major}
}

// refAssign 按 C.04.02 §2 的规则表（第一条命中就返回）、§3、R3 算出期望结果。只用来核对向量写对了；
// 正式实现在 E.03.02（前端 W.05 另有一份），用同一组向量。
type refAssign struct {
	types map[string]assignType // "名称@主版本"
	tmpl  map[string]string
}

func (r refAssign) reject(rule, src, dst, producer string) assignResult {
	d := parseRef(dst)
	name := dst
	if d.kind == "named" {
		name = d.name
	}
	msg := strings.NewReplacer("{src}", src, "{dst}", dst, "{dst_name}", name, "{producer}", producer).Replace(r.tmpl[rule])
	return assignResult{Rule: rule, Reason: msg}
}

func (r refAssign) ancestors(name string, major int) []string {
	var out []string
	cur, ok := r.types[name+"@"+strconv.Itoa(major)]
	for ok && cur.Extends != "" && len(out) < 16 {
		p := parseRef(cur.Extends)
		key := p.name + "@" + strconv.Itoa(p.major)
		out = append(out, key)
		cur, ok = r.types[key]
	}
	return out
}

func (r refAssign) assign(src, dst string, v assignVector, inList bool) assignResult {
	s, d := parseRef(src), parseRef(dst)
	if d.kind == "any" {
		return assignResult{OK: true, Rule: "C0"}
	}
	if s.kind == "any" {
		return r.reject("C1", src, dst, "")
	}
	if d.kind == "named" {
		if p := r.types[d.name+"@"+strconv.Itoa(d.major)].Producer; p != "" && v.SrcNode != p {
			return r.reject("C2", src, dst, p)
		}
	}
	if s.kind == "named" && d.kind == "named" {
		dkey, skey := d.name+"@"+strconv.Itoa(d.major), s.name+"@"+strconv.Itoa(s.major)
		if s.name == d.name {
			if s.major == d.major {
				return assignResult{OK: true, Rule: "C3"}
			}
			if slices.Contains(r.types[dkey].MigrateFrom, strconv.Itoa(s.major)) {
				return assignResult{OK: true, Rule: "C4", Migrate: s.name + "@" + strconv.Itoa(s.major) + "→" + strconv.Itoa(d.major)}
			}
			return r.reject("C4", src, dst, "")
		}
		if slices.Contains(r.ancestors(s.name, s.major), dkey) {
			return assignResult{OK: true, Rule: "C5"}
		}
		if slices.Contains(r.ancestors(d.name, d.major), skey) {
			return r.reject("C6", src, dst, "")
		}
	}
	if s.kind == "list" && d.kind != "list" && !v.DstContainer && !inList {
		if e := r.assign(s.elem, dst, v, true); e.OK {
			return assignResult{OK: true, Expand: true, Migrate: e.Migrate, Rule: "C7"}
		}
	}
	if s.kind != "list" && d.kind == "list" {
		return r.reject("C8", src, dst, "")
	}
	if s.kind == "list" && d.kind == "list" {
		e := r.assign(s.elem, d.elem, v, true)
		return assignResult{OK: e.OK, Migrate: e.Migrate, Rule: "C9", Reason: e.Reason}
	}
	return r.reject("C10", src, dst, "")
}

func readAssignTemplates(t *testing.T) map[string]string {
	t.Helper()
	var tmpl map[string]string
	readYAML(t, "schema/builtin/messages/assign.zh.yaml", &tmpl)
	return tmpl
}

func newRefAssign(t *testing.T, f assignFile) refAssign {
	t.Helper()
	r := refAssign{types: map[string]assignType{}, tmpl: readAssignTemplates(t)}
	for _, ty := range f.Types {
		key := ty.Name + "@" + strconv.Itoa(ty.Major)
		if _, dup := r.types[key]; dup {
			t.Errorf("types 里 %s 重复", key)
		}
		r.types[key] = ty
	}
	return r
}

func TestAssignVectors(t *testing.T) {
	t.Run("C0402-T1 向量格式", func(t *testing.T) {
		s := repoSet(t)
		f := readAssignVectors(t)
		r := newRefAssign(t, f)
		for _, ty := range f.Types {
			if ty.Extends != "" {
				mustValid(t, s, "types.schema.json#/$defs/TypeRef", `"`+ty.Extends+`"`)
			}
		}
		count := map[string]int{}
		for _, v := range f.Vectors {
			mustValid(t, s, "errors.schema.json#/$defs/Assign", string(v.Want))
			for _, tr := range []string{v.Src, v.Dst} {
				mustValid(t, s, "types.schema.json#/$defs/TypeRef", `"`+tr+`"`)
				for p := parseRef(tr); p.kind != "any"; p = parseRef(p.elem) {
					if p.kind == "named" {
						if _, ok := r.types[p.name+"@"+strconv.Itoa(p.major)]; !ok {
							t.Errorf("%s：类型 %s 不在 types 里", v.Name, tr)
						}
						break
					}
					if p.kind != "list" {
						t.Errorf("%s：%s 写法不对", v.Name, tr)
						break
					}
				}
			}
			var want assignResult
			if err := json.Unmarshal(v.Want, &want); err != nil {
				t.Fatal(err)
			}
			count[want.Rule]++
			if got := r.assign(v.Src, v.Dst, v, false); !reflect.DeepEqual(got, want) {
				t.Errorf("%s（%s → %s）：按 C.04.02 §2 应为 %+v，向量里是 %+v", v.Name, v.Src, v.Dst, got, want)
			}
			if want.OK == (want.Reason != "") {
				t.Errorf("%s：可连时不写 reason，拒绝时必须写", v.Name)
			}
		}
		if len(f.Vectors) < 40 {
			t.Errorf("至少 40 条，实际 %d 条（C.04.02 §6）", len(f.Vectors))
		}
		for i := 0; i <= 10; i++ {
			if rule := "C" + strconv.Itoa(i); count[rule] < 3 {
				t.Errorf("%s 只有 %d 条，每条规则至少 3 条", rule, count[rule])
			}
		}
	})
	t.Run("C0402-T2 INV-07", func(t *testing.T) {
		f := readAssignVectors(t)
		found := false
		for _, v := range f.Vectors {
			var want assignResult
			_ = json.Unmarshal(v.Want, &want)
			if v.Src == "资源引用@1" && v.Dst == "上下文包@1" && v.SrcNode == "资源" {
				found = true
				if want.OK || want.Rule != "C2" || want.Reason != "上下文包只能来自上下文装配节点" {
					t.Errorf("资源接 context 应当按 C2 拒绝，提示「上下文包只能来自上下文装配节点」：%+v", want)
				}
			}
		}
		if !found {
			t.Error("没有「资源引用@1 → 上下文包@1、src_node 为资源」这一条（C.04.02 §6 验收 2）")
		}
	})
	t.Run("C0402-T3 提示文案", func(t *testing.T) {
		tmpl := readAssignTemplates(t)
		if got, want := sortedKeys(tmpl), []string{"C1", "C10", "C2", "C4", "C6", "C8"}; !reflect.DeepEqual(got, want) {
			t.Errorf("assign.zh.yaml 的规则：%v，应当恰好是 C1、C2、C4、C6、C8、C10", got)
		}
		for rule, msg := range tmpl {
			if strings.TrimSpace(msg) == "" {
				t.Errorf("%s 的文案是空的", rule)
			}
			for _, m := range regexp.MustCompile(`\{([a-z_]+)\}`).FindAllStringSubmatch(msg, -1) {
				if !slices.Contains([]string{"src", "dst", "dst_name", "producer"}, m[1]) {
					t.Errorf("%s 用了不认识的变量 {%s}", rule, m[1])
				}
			}
		}
		// 每条拒绝向量的 reason 都是某条模板代入变量后的文字（C9 用元素比较的规则和元素类型），由 T1 的逐条比较保证
	})
}
