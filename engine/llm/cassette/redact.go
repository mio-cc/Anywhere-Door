package cassette

import (
	"encoding/json"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

// redactBody 脱敏请求体（S.03-R7、Q.06-R6）：传入的密钥原文和像密钥的串（规则同 llm.Redact）换成 [REDACTED]。
// 结果仍是 JSON；请求体不是 JSON 时存成 JSON 字符串。
func redactBody(body []byte, keys []string) json.RawMessage {
	if len(body) == 0 {
		return nil
	}
	s := llm.Redact(string(body), keys...)
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	b, _ := json.Marshal(s)
	return b
}
