package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

// Risk 是工具的风险等级（S.04 §3）。
type Risk uint8

// 风险等级的取值。
const (
	RiskL0 Risk = iota // 只读
	RiskL1             // 有副作用，可撤销
	RiskL2             // 对外，可能可撤销
	RiskL3             // 对外，不可撤销
)

// ParseRisk 解析 "L0" … "L3"。
func ParseRisk(s string) (Risk, error) {
	switch s {
	case "L0":
		return RiskL0, nil
	case "L1":
		return RiskL1, nil
	case "L2":
		return RiskL2, nil
	case "L3":
		return RiskL3, nil
	}
	return 0, fmt.Errorf("风险等级写法不对：%q（只能是 L0–L3）", s)
}

func (r Risk) String() string {
	if r <= RiskL3 {
		return "L" + strconv.Itoa(int(r))
	}
	return "L?"
}

// Action 是授权判定用的动作（C.09）。
func (r Risk) Action() gen.Action {
	switch r {
	case RiskL0:
		return gen.ActToolL0
	case RiskL1:
		return gen.ActToolL1
	case RiskL2:
		return gen.ActToolL2
	}
	return gen.ActToolL3
}

// Def 是工具的定义（E.11.01 §4）。
type Def struct {
	ID           string          // builtin:ws_read | mcp:<服务>/<工具> | ext:<包>/<节点> | skill:<名>
	Name         string          // 给模型看的名字 ^[a-z0-9_]{1,64}$
	Description  string          // 给模型看的用途
	Input        json.RawMessage // 参数的 JSON Schema
	Risk         Risk            // 注册时声明，不能低于 S.04 §3 的下限
	Idempotent   bool            // 外部系统本身支持幂等键
	Timeout      time.Duration   // 0 = 60 秒
	Pool         string          // "" = tool:<ID>
	TaintsResult bool            // 结果含外部内容
}

// Call 是一次调用的输入。
type Call struct {
	Args    json.RawMessage
	IdemKey string
	Taint   bool
}

// Impl 是工具的实现。Invoke 出错时最好返回 *Error，写明失败类别。
type Impl interface {
	Def() Def
	Invoke(ctx context.Context, c Call) (Raw, error)
}
