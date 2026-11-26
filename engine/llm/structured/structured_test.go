package structured

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/engine/llm/fake"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"pgregory.net/rapid"
)

func sameJSON(t *testing.T, got json.RawMessage, want string) bool {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(want), &b); err != nil {
		t.Fatalf("期望值不是 JSON：%s", want)
	}
	return reflect.DeepEqual(a, b)
}

func TestRepair(t *testing.T) {
	t.Run("E1006-T1 修复向量", func(t *testing.T) {
		cases := []struct{ in, want, repair string }{
			{"```json\n{\"a\": 1}\n```", `{"a":1}`, RepairCodeFence},
			{"结果如下：\n```\n[1, 2]\n```\n以上。", `[1,2]`, RepairCodeFence},
			{"``` {\"a\": \"x\"} ```", `{"a":"x"}`, RepairCodeFence},
			{`{"a": 1, "b": [1, 2`, `{"a":1,"b":[1,2]}`, RepairClose},
			{`{"a": {"b": "x`, `{"a":{"b":"x"}}`, RepairClose},
			{`[{"a": 1}, {"a": 2`, `[{"a":1},{"a":2}]`, RepairClose},
			{`{"a": 1,}`, `{"a":1}`, RepairTrailingComma},
			{`[1, 2, ]`, `[1,2]`, RepairTrailingComma},
			{`{"a": [1,], "b": 2,}`, `{"a":[1],"b":2}`, RepairTrailingComma},
			{`{'a': 'x'}`, `{"a":"x"}`, RepairSingleQuote},
			{`{'a': "it's"}`, `{"a":"it's"}`, RepairSingleQuote},
			{`{'say': 'he said "hi"'}`, `{"say":"he said \"hi\""}`, RepairSingleQuote},
			{"{\"a\": 1 // 数量\n}", `{"a":1}`, RepairComment},
			{`{/* 说明 */ "a": 1}`, `{"a":1}`, RepairComment},
			{`{"url": "http://x.com/a", /* c */ "b": 2}`, `{"url":"http://x.com/a","b":2}`, RepairComment},
		}
		for _, c := range cases {
			raw, reps, ok := Repair(c.in)
			if !ok || !sameJSON(t, raw, c.want) || !slices.Contains(reps, c.repair) {
				t.Errorf("Repair(%q) = %s %v %v，应当是 %s 且记了 %s", c.in, raw, reps, ok, c.want, c.repair)
			}
		}
		raw, reps, ok := Repair(`好的，结果是 {"a": 1} 。`)
		if !ok || !sameJSON(t, raw, `{"a":1}`) || !slices.Contains(reps, RepairExtract) {
			t.Errorf("前后有别的文字时取出 JSON 并记 extract：%s %v %v", raw, reps, ok)
		}
		for _, bad := range []string{"没有 JSON", `{"a": }`, ""} {
			if _, _, ok := Repair(bad); ok {
				t.Errorf("Repair(%q) 应当失败", bad)
			}
		}
	})
}

const outSchema = `{"type":"object","properties":{"n":{"type":"integer"},"tags":{"type":"array","items":{"type":"string"}},` +
	`"lang":{"type":"string","default":"zh"}},"required":["n"],"additionalProperties":false}`

func compile(t *testing.T, s string) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	const url = "https://anydoor.local/out.schema.json"
	c := jsonschema.NewCompiler()
	if err := c.AddResource(url, doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile(url)
	if err != nil {
		t.Fatal(err)
	}
	return sch
}

func userReq() llm.Req {
	return llm.Req{Model: "m", Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartText, Text: "给出结果"}}}}}
}

func run(t *testing.T, caps llm.Caps, s Strategy, resp *llm.Resp) (Result, llm.Req, error) {
	t.Helper()
	out := llm.OutputSpec{Name: "result", Schema: json.RawMessage(outSchema)}
	c := fake.New(caps, fake.Step{Resp: resp})
	st, err := c.Complete(context.Background(), Prepare(userReq(), s, out))
	if err != nil {
		t.Fatal(err)
	}
	r, err := llm.Collect(st)
	if err != nil {
		t.Fatal(err)
	}
	res, derr := Decode(r, s, out, compile(t, outSchema))
	return res, c.Requests()[0], derr
}

func TestStrategies(t *testing.T) {
	t.Run("E1006-T2 策略选择", func(t *testing.T) {
		if Pick(llm.Caps{NativeSchema: true, Tools: true}) != Native || Pick(llm.Caps{Tools: true}) != ForcedTool || Pick(llm.Caps{}) != TextParse {
			t.Error("Pick：原生 → 强制工具 → 文本解析")
		}
		if s, ok := Next(Native, llm.Caps{Tools: true}); !ok || s != ForcedTool {
			t.Error("Native 降级到 ForcedTool")
		}
		if s, ok := Next(Native, llm.Caps{}); !ok || s != TextParse {
			t.Error("不支持工具时 Native 直接降级到 TextParse")
		}
		if s, ok := Next(ForcedTool, llm.Caps{}); !ok || s != TextParse {
			t.Error("ForcedTool 降级到 TextParse")
		}
		if _, ok := Next(TextParse, llm.Caps{}); ok {
			t.Error("TextParse 没有下一级")
		}

		res, req, err := run(t, llm.Caps{NativeSchema: true}, Native, &llm.Resp{Structured: json.RawMessage(`{"n":1}`)})
		if err != nil || !sameJSON(t, res.JSON, `{"n":1,"lang":"zh"}`) || req.Output == nil {
			t.Errorf("Native：%s %v %+v", res.JSON, err, req.Output)
		}
		res, req, err = run(t, llm.Caps{Tools: true}, ForcedTool,
			&llm.Resp{ToolCalls: []llm.ToolCall{{ID: "1", Name: llm.EmitPrefix + "result", Args: json.RawMessage(`{"n":2,"tags":["a"]}`)}}})
		if err != nil || !sameJSON(t, res.JSON, `{"n":2,"tags":["a"],"lang":"zh"}`) || req.Output != nil ||
			req.ToolChoice != llm.EmitPrefix+"result" || len(req.Tools) != 1 || req.Tools[0].Name != req.ToolChoice {
			t.Errorf("ForcedTool：%s %v %+v", res.JSON, err, req)
		}
		res, req, err = run(t, llm.Caps{}, TextParse, &llm.Resp{Text: "好的：\n```json\n{\"n\": 3}\n```"})
		if err != nil || !sameJSON(t, res.JSON, `{"n":3,"lang":"zh"}`) || !slices.Contains(res.Repairs, RepairCodeFence) ||
			req.Messages[0].Role != llm.RoleSystem || !strings.Contains(req.Messages[0].Parts[0].Text, "只输出一个 JSON 对象") {
			t.Errorf("TextParse：%s %v %v %+v", res.JSON, err, res.Repairs, req.Messages)
		}
		sys := userReq()
		sys.Messages = append([]llm.Message{{Role: llm.RoleSystem, Parts: []llm.Part{{Type: llm.PartText, Text: "底座"}}}}, sys.Messages...)
		if p := Prepare(sys, TextParse, llm.OutputSpec{Name: "r"}); len(p.Messages) != 2 || len(p.Messages[0].Parts) != 2 || len(sys.Messages[0].Parts) != 1 {
			t.Error("已有 system 消息时加在它末尾，且不改原请求")
		}

		out := llm.OutputSpec{Name: "result", Schema: json.RawMessage(outSchema)}
		sch := compile(t, outSchema)
		res, err = Decode(&llm.Resp{Structured: json.RawMessage(`{"n":"3","tags":"a"}`)}, Native, out, sch)
		if err != nil || !sameJSON(t, res.JSON, `{"n":3,"tags":["a"],"lang":"zh"}`) ||
			!slices.Contains(res.Repairs, RepairNumberString) || !slices.Contains(res.Repairs, RepairSingleToArray) || !slices.Contains(res.Repairs, "default:/lang") {
			t.Errorf("宽松修正：%s %v %v", res.JSON, res.Repairs, err)
		}
		res, err = Decode(&llm.Resp{Structured: json.RawMessage(`{"n":"x"}`)}, Native, out, sch)
		var le *llm.Error
		if !errors.As(err, &le) || le.Code != CodeSchemaMismatch || le.Class != "format_invalid" || len(res.Errors) == 0 || res.Errors[0].Path != "/n" {
			t.Errorf("校验不通过：%v %+v", err, res.Errors)
		}
		fb := Feedback(res.Errors)
		if fb.Role != llm.RoleUser || !strings.Contains(fb.Parts[0].Text, "/n") {
			t.Errorf("Feedback 是 user 消息且写了位置：%+v", fb)
		}
		if _, err := Decode(&llm.Resp{Text: "不知道"}, TextParse, out, nil); !errors.As(err, &le) || le.Code != CodeNoJSON {
			t.Errorf("找不到 JSON：%v", err)
		}
	})
}

func jsonValue(depth int) *rapid.Generator[any] {
	return rapid.Custom(func(t *rapid.T) any {
		if depth == 0 || rapid.IntRange(0, 2).Draw(t, "leaf") == 0 {
			switch rapid.IntRange(0, 3).Draw(t, "kind") {
			case 0:
				return rapid.String().Draw(t, "s")
			case 1:
				return rapid.Int().Draw(t, "i")
			case 2:
				return rapid.Bool().Draw(t, "b")
			default:
				return nil
			}
		}
		if rapid.Bool().Draw(t, "array") {
			return rapid.SliceOfN(jsonValue(depth-1), 0, 4).Draw(t, "a")
		}
		return rapid.MapOfN(rapid.String(), jsonValue(depth-1), 0, 4).Draw(t, "m")
	})
}

func TestRepairKeepsValid(t *testing.T) {
	t.Run("E1006-T3 合法输入不变（rapid）", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			obj := rapid.MapOfN(rapid.String(), jsonValue(2), 0, 5).Draw(t, "obj")
			b, err := json.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}
			if rapid.Bool().Draw(t, "indent") {
				b, _ = json.MarshalIndent(obj, "", "  ")
			}
			raw, reps, ok := Repair(string(b))
			var x, y any
			_ = json.Unmarshal(b, &x)
			_ = json.Unmarshal(raw, &y)
			if !ok || len(reps) != 0 || !reflect.DeepEqual(x, y) {
				t.Fatalf("合法 JSON 不应当被改：%s → %s %v %v", b, raw, reps, ok)
			}
		})
	})
}
