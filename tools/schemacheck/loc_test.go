package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type pathIter struct {
	Round   int `json:"round,omitempty"`
	Event   int `json:"event,omitempty"`
	Fan     int `json:"fan,omitempty"`
	Attempt int `json:"attempt,omitempty"`
}

type pathStep struct {
	NodeID    string          `json:"node_id"`
	Label     string          `json:"label"`
	Iter      json.RawMessage `json:"iter"`
	SameLabel bool            `json:"same_label"`
}

type pathVector struct {
	Name  string     `json:"name"`
	Chain []pathStep `json:"chain"`
	Path  string     `json:"path"`
	Key   string     `json:"key"`
}

// refPathKey 按 C.06.05 §2、R1、R2、R5 由 chain 算出路径和唯一键。只用来核对向量本身写对了；
// 正式实现在 E.09.01（前后端各一份），用同一组向量。
func refPathKey(chain []pathStep) (string, string, error) {
	fullwidth := strings.NewReplacer("/", "／", "#", "＃", "@", "＠", "~", "～")
	parts := []string{"根图"}
	var keys []string
	for i, st := range chain {
		var it pathIter
		if err := json.Unmarshal(st.Iter, &it); err != nil {
			return "", "", err
		}
		seg := fullwidth.Replace(st.Label)
		if st.SameLabel {
			seg += "~" + st.NodeID[len(st.NodeID)-4:]
		}
		last := i == len(chain)-1
		if !last { // 容器：#r、#e、#N；不写尝试次数
			var ks []string
			if it.Round > 0 {
				seg += fmt.Sprintf("#r%d", it.Round)
				ks = append(ks, fmt.Sprintf("r%d", it.Round))
			}
			if it.Event > 0 {
				seg += fmt.Sprintf("#e%d", it.Event)
				ks = append(ks, fmt.Sprintf("e%d", it.Event))
			}
			if it.Fan > 0 {
				seg += fmt.Sprintf("#%d", it.Fan)
				ks = append(ks, fmt.Sprintf("f%d", it.Fan))
			}
			keys = append(keys, st.NodeID+":"+strings.Join(ks, "."))
		} else {
			attempt := max(it.Attempt, 1)
			self := fmt.Sprintf("a%d", attempt)
			if it.Fan > 0 {
				seg += fmt.Sprintf("#%d", it.Fan)
				self = fmt.Sprintf("f%d.", it.Fan) + self
			}
			if attempt > 1 {
				seg += fmt.Sprintf("@%d", attempt)
			}
			keys = append([]string{st.NodeID}, append(keys, self)...)
		}
		parts = append(parts, seg)
	}
	return strings.Join(parts, "/"), strings.Join(keys, "|"), nil
}

func TestLoc(t *testing.T) {
	t.Run("C0605-T1 Loc 与 Iter", func(t *testing.T) {
		s := repoSet(t)
		manifestEntry(t, s, "loc_loop_executor.json")
		if e := manifestEntry(t, s, "invalid/loc_round_zero.json"); e.ErrorAt == nil || *e.ErrorAt != "/iter/round" {
			t.Errorf("清单里 loc_round_zero.json 的 error_at：%+v", e)
		}
		const loc = "common.schema.json#/$defs/Loc"
		const id = `"01JDN0000000000000000000C1"`
		mustValid(t, s, loc, `{"node_id": `+id+`, "path": "根图/执行者"}`)
		mustInvalid(t, s, loc, `{"node_id": `+id+`, "path": "根图", "scopes": [{"node_id": `+id+`, "x": 1}]}`, "/scopes/0")
		mustInvalid(t, s, loc, `{"node_id": `+id+`, "path": "根图", "scopes": [{"iter": {"round": 1}}]}`, "/scopes/0")
		mustInvalid(t, s, loc, `{"path": "根图"}`, "")
		mustInvalid(t, s, loc, `{"node_id": `+id+`, "path": "`+strings.Repeat("a", 1001)+`"}`, "/path")
		const iter = "common.schema.json#/$defs/Iter"
		mustValid(t, s, iter, `{"round": 2, "fan": 3, "attempt": 1}`)
		mustValid(t, s, iter, `{}`)
		for _, f := range []string{"round", "event", "fan", "attempt"} {
			mustInvalid(t, s, iter, `{"`+f+`": 0}`, "/"+f)
			mustInvalid(t, s, iter, `{"`+f+`": 1.5}`, "/"+f)
		}
		mustInvalid(t, s, iter, `{"retry": 1}`, "")
	})
	t.Run("C0605-T2 路径向量", func(t *testing.T) {
		s := repoSet(t)
		b, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", "path_vectors.json"))
		if err != nil {
			t.Fatal(err)
		}
		var vs []pathVector
		dec := json.NewDecoder(strings.NewReader(string(b)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&vs); err != nil {
			t.Fatalf("path_vectors.json：%v", err)
		}
		if len(vs) < 8 {
			t.Errorf("至少 8 条，实际 %d 条（C.06.05 §4）", len(vs))
		}
		ulid := `[0-9A-HJKMNP-TV-Z]{26}`
		keyRe := regexp.MustCompile(`^` + ulid + `(\|` + ulid + `:[ref]\d+(\.[ref]\d+)*)*\|(f\d+\.)?a\d+$`)
		covered := map[string]bool{}
		for _, v := range vs {
			if v.Name == "" || len(v.Chain) == 0 {
				t.Errorf("每条要有 name 和非空的 chain：%+v", v)
				continue
			}
			for i, st := range v.Chain {
				mustValid(t, s, "common.schema.json#/$defs/Ulid", `"`+st.NodeID+`"`)
				if len(st.Iter) == 0 {
					t.Errorf("%s：chain[%d] 要写 iter（可以是 {}）", v.Name, i)
					continue
				}
				mustValid(t, s, "common.schema.json#/$defs/Iter", string(st.Iter))
			}
			if !keyRe.MatchString(v.Key) {
				t.Errorf("%s：key %q 不符合 C.06.05-R5 的写法", v.Name, v.Key)
			}
			path, key, err := refPathKey(v.Chain)
			if err != nil {
				t.Errorf("%s：%v", v.Name, err)
				continue
			}
			if path != v.Path || key != v.Key {
				t.Errorf("%s：按 C.06.05 算出 path %q、key %q；向量里是 %q、%q", v.Name, path, key, v.Path, v.Key)
			}
			last := v.Chain[len(v.Chain)-1]
			var it pathIter
			_ = json.Unmarshal(last.Iter, &it)
			covered["根图下的直接节点"] = covered["根图下的直接节点"] || len(v.Chain) == 1
			covered["#r"] = covered["#r"] || strings.Contains(v.Path, "#r")
			covered["#e"] = covered["#e"] || strings.Contains(v.Path, "#e")
			covered["扇出份号"] = covered["扇出份号"] || regexp.MustCompile(`#\d+/`).MatchString(v.Path)
			covered["C7 展开份号"] = covered["C7 展开份号"] || it.Fan > 0
			covered["@N 重试"] = covered["@N 重试"] || it.Attempt > 1
			covered["~ 重名"] = covered["~ 重名"] || last.SameLabel
			covered["全角替换"] = covered["全角替换"] || strings.ContainsAny(v.Path, "／＃＠～")
		}
		for _, c := range []string{"根图下的直接节点", "#r", "#e", "扇出份号", "C7 展开份号", "@N 重试", "~ 重名", "全角替换"} {
			if !covered[c] {
				t.Errorf("向量没有覆盖：%s（C.06.05 §4）", c)
			}
		}
	})
}
