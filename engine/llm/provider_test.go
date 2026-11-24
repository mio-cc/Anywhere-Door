package llm

import (
	"strings"
	"testing"
)

func TestParseProviders(t *testing.T) {
	t.Run("E1001-T3 提供方配置校验", func(t *testing.T) {
		ok := `{"name":"mistral","protocol":"chat_completions","base_url":"https://api.mistral.ai/v1","auth":{"type":"bearer","secret":"mistral"},` +
			`"models":[{"id":"small","upstream":"mistral-small-latest"}],"override":[{"op":"set","path":"safe_prompt","value":false}]}`
		ps, warns, err := ParseProviders([]byte("[" + ok + `,{"name":"local","protocol":"chat_completions","base_url":"http://127.0.0.1:11434/v1","auth":{"type":"none"}}]`))
		if err != nil || len(ps) != 2 || len(warns) != 0 {
			t.Fatalf("合法配置：%v %v %v", ps, warns, err)
		}
		if ps[0].Models[0].UpstreamName() != "mistral-small-latest" || ps[1].Timeout() != DefaultTimeout {
			t.Errorf("解析结果不对：%+v", ps)
		}

		base := func(extra string) string {
			return `{"name":"p","protocol":"chat_completions","base_url":"https://x.example/v1","auth":{"type":"none"}` + extra + `}`
		}
		bad := []struct{ name, json, want string }{
			{"名字不合规", `[{"name":"Bad Name","protocol":"chat_completions","base_url":"https://x.example","auth":{"type":"none"}}]`, "name"},
			{"重名", "[" + base("") + "," + base("") + "]", "重名"},
			{"未知协议", `[{"name":"p","protocol":"grpc","base_url":"https://x.example","auth":{"type":"none"}}]`, "protocol"},
			{"base_url 不是绝对地址", `[{"name":"p","protocol":"chat_completions","base_url":"/v1","auth":{"type":"none"}}]`, "base_url"},
			{"header 鉴权缺 name", `[{"name":"p","protocol":"chat_completions","base_url":"https://x.example","auth":{"type":"header","secret":"k"}}]`, "auth"},
			{"query 鉴权缺 name", `[{"name":"p","protocol":"chat_completions","base_url":"https://x.example","auth":{"type":"query","secret":"k"}}]`, "auth"},
			{"要密钥却没有 secret", `[{"name":"p","protocol":"chat_completions","base_url":"https://x.example","auth":{"type":"bearer"}}]`, "secret"},
			{"缺省鉴权也要 secret", `[{"name":"p","protocol":"anthropic_messages","base_url":"https://x.example","auth":{}}]`, "secret"},
			{"未知鉴权", `[{"name":"p","protocol":"chat_completions","base_url":"https://x.example","auth":{"type":"oauth","secret":"k"}}]`, "auth"},
			{"override 的 op 不认识", "[" + base(`,"override":[{"op":"merge","path":"a"}]`) + "]", "override[0]"},
			{"rename 缺 to", "[" + base(`,"override":[{"op":"rename","path":"a"}]`) + "]", "override[0]"},
			{"set 缺 value", "[" + base(`,"override":[{"op":"set","path":"a"}]`) + "]", "override[0]"},
			{"path 不是点分路径", "[" + base(`,"override":[{"op":"delete","path":"a..b"}]`) + "]", "override[0]"},
			{"rename 的 to 不是点分路径", "[" + base(`,"override":[{"op":"rename","path":"a","to":"b..c"}]`) + "]", "to"},
			{"模型 id 重名", "[" + base(`,"models":[{"id":"a"},{"id":"a"}]`) + "]", "models[1].id"},
			{"模型 id 为空", "[" + base(`,"models":[{"upstream":"x"}]`) + "]", "models[0].id"},
			{"路径名不认识", "[" + base(`,"paths":{"chat":"/c"}`) + "]", "paths"},
			{"不认识的字段", "[" + base(`,"colour":"red"`) + "]", "colour"},
			{"代理地址不对", "[" + base(`,"proxy":"ftp://x"`) + "]", "proxy"},
		}
		for _, tc := range bad {
			_, _, err := ParseProviders([]byte(tc.json))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s：应当报错并写明 %q，实际 %v", tc.name, tc.want, err)
			}
		}
		// 错误里写明是第几个提供方
		_, _, err = ParseProviders([]byte("[" + base("") + `,{"name":"q","protocol":"x","base_url":"https://x.example","auth":{"type":"none"}}]`))
		if err == nil || !strings.Contains(err.Error(), "#2（q）") {
			t.Errorf("错误应当写明是第 2 个提供方 q：%v", err)
		}

		// 预留字段能解析，各记一条 warn
		_, warns, err = ParseProviders([]byte("[" + base(`,"keys":["a","b"],"priority":1,"weight":3,"fallback":["q"]`) + "]"))
		if err != nil || len(warns) != 4 {
			t.Errorf("keys、priority、weight、fallback 各记一条 warn：%v %v", warns, err)
		}
		for _, k := range []string{"keys", "priority", "weight", "fallback"} {
			if !strings.Contains(strings.Join(warns, "\n"), k) {
				t.Errorf("warn 里应当提到 %s：%v", k, warns)
			}
		}
		// http:// 加远程主机加鉴权记 warn；本机不记
		_, warns, _ = ParseProviders([]byte(`[{"name":"p","protocol":"chat_completions","base_url":"http://10.0.0.5:8000/v1","auth":{"type":"bearer","secret":"k"}}]`))
		if len(warns) != 1 || !strings.Contains(warns[0], "明文") {
			t.Errorf("http:// 远程地址带鉴权应当记 warn：%v", warns)
		}
		_, warns, _ = ParseProviders([]byte(`[{"name":"p","protocol":"chat_completions","base_url":"http://localhost:8000/v1","auth":{"type":"bearer","secret":"k"}}]`))
		if len(warns) != 0 {
			t.Errorf("本机地址不记 warn：%v", warns)
		}
		// 空配置
		if ps, _, err := ParseProviders(nil); err != nil || ps != nil {
			t.Errorf("空配置：%v %v", ps, err)
		}
	})
}
