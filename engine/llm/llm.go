// Package llm 是 Agent 节点和编排者调用模型的唯一通道（设计文档 E.10、E.10.01，INV-05）。
//
// 按接口协议实现，不按厂商实现（ADR-011）：代码只认几种主流协议（engine/llm/protocol/<名>），
// 接哪一家、地址、鉴权、请求头、模型名、请求体怎么改、能力是什么，全部写在模型提供方的配置里（ProviderConfig）。
// 厂商差异写成数据（quirks 表，E.10.05），本包和协议包里不比较厂商名、模型名（E.10-R2）。
package llm

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
)

// 已知的接口协议（E.10.01 协议表）。加一种协议：在 engine/llm/protocol/<名>/ 实现 Protocol，
// 名字加进这里，在组装处传给 NewRegistry（E.10-R7）。
const (
	ProtoChat      = "chat_completions"
	ProtoResponses = "responses"
	ProtoMessages  = "anthropic_messages"
)

// KnownProtocols 是配置校验认的协议名。
var KnownProtocols = []string{ProtoChat, ProtoResponses, ProtoMessages}

// Role 是消息的角色。
type Role string

// Role 的取值。
const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Part.Type 的取值。
const (
	PartText  = "text"
	PartImage = "image"
	PartFile  = "file"
)

// Resp.Finish 的取值。
const (
	FinishStop      = "stop"
	FinishLength    = "length"
	FinishToolCalls = "tool_calls"
	FinishFiltered  = "filtered"
)

// Part 是消息里的一段内容。
type Part struct {
	Type string // text | image | file
	Text string
	Mime string
	Data []byte // image / file 的内容：上层读出工作区引用后填，协议层不访问工作区
}

// Message 是一条消息。
type Message struct {
	Role       Role
	Parts      []Part
	ToolCalls  []ToolCall // assistant
	ToolCallID string     // tool
	Cache      bool       // 提示缓存断点（协议支持时才生效）
}

// ToolSpec 是给模型的一个工具。
type ToolSpec struct {
	Name, Description string
	Schema            json.RawMessage
}

// ToolCall 是模型发起的一次工具调用。
type ToolCall struct {
	ID, Name string
	Args     json.RawMessage
}

// OutputSpec 要求模型按 schema 输出结构化结果（E.10.01 §5.1 第 5 步）。
type OutputSpec struct {
	Name   string
	Schema json.RawMessage
	Strict bool
}

// Req 是统一的请求。
type Req struct {
	Model       string // 提供方配置里的模型 id（ModelEntry.ID）
	Messages    []Message
	Tools       []ToolSpec
	ToolChoice  string // auto | none | required | <工具名>
	Output      *OutputSpec
	Temperature *float32
	TopP        *float32
	MaxTokens   int
	Reasoning   string // "" | off | minimal | low | medium | high | xhigh | max（U.09.06-R5），按 E.10.05 映射
	Stop        []string
	Metadata    map[string]string // 只给调用方自己用，不发给上游
}

// Usage 是一次调用的用量（token 数）。
type Usage struct{ In, Cached, Out, Reasoning int }

// Resp 是一次调用的完整结果。
type Resp struct {
	Text       string
	Thinking   string     // 上游返回的思考文本（有的协议有），不回传给模型
	ToolCalls  []ToolCall // 不含结构化输出用的强制工具
	Structured json.RawMessage
	Finish     string // stop | length | tool_calls | filtered
	Usage      Usage
	Model      string          // 上游实际返回的模型名
	Raw        json.RawMessage // 上游响应的原始数据（SSE 的 data 逐条组成的 JSON 数组，最多 1 MB），供回放和排查
}

// ToolCallDelta 是流式到达的工具调用片段。
type ToolCallDelta struct {
	Index    int    // 同一次响应里第几个工具调用
	ID, Name string // 第一次出现时给
	Args     string // 参数片段，按到达顺序拼接
}

// Delta 是流里的一段。
type Delta struct {
	Text, Thinking string
	ToolCall       *ToolCallDelta
	Done           bool
	Resp           *Resp // Done 时给完整结果
}

// Stream 是一次调用的流式结果。
type Stream interface {
	Next() (Delta, error) // 结束时返回 io.EOF
	Close() error
}

// Client 是一个提供方的一个模型。只有 RunCtx.LLM() 能拿到（E.10-R1）。
type Client interface {
	Complete(ctx context.Context, r Req) (Stream, error)
	Caps() Caps
}

// Caps 是一个模型的能力（E.10.01 §4.2）。
type Caps struct {
	Window, MaxOutput                         int
	Tools, ParallelTools, Vision, PromptCache bool
	NativeSchema                              bool              // 协议原生的结构化输出（json_schema 等）
	Efforts                                   []string          // 支持的思考强度；空 = 不支持调节
	EffortMap                                 map[string]string // 不支持的强度改用哪一级
	EffortBudget                              map[string]int    // 按 token 预算表达强度的协议用（anthropic_messages）
	Tokenizer                                 string
}

// CapsOverride 覆盖 Caps 里写了的字段：指针为 nil、切片和 map 为 nil 的不覆盖。
type CapsOverride struct {
	Window        *int              `json:"window,omitempty"`
	MaxOutput     *int              `json:"max_output,omitempty"`
	Tools         *bool             `json:"tools,omitempty"`
	ParallelTools *bool             `json:"parallel_tools,omitempty"`
	Vision        *bool             `json:"vision,omitempty"`
	PromptCache   *bool             `json:"prompt_cache,omitempty"`
	NativeSchema  *bool             `json:"native_schema,omitempty"`
	Efforts       []string          `json:"efforts,omitempty"`
	EffortMap     map[string]string `json:"effort_map,omitempty"`
	EffortBudget  map[string]int    `json:"effort_budget,omitempty"`
	Tokenizer     *string           `json:"tokenizer,omitempty"`
}

// Apply 返回用 o 覆盖之后的能力；o 为 nil 时原样返回。
func (c Caps) Apply(o *CapsOverride) Caps {
	if o == nil {
		return c
	}
	if o.Window != nil {
		c.Window = *o.Window
	}
	if o.MaxOutput != nil {
		c.MaxOutput = *o.MaxOutput
	}
	if o.Tools != nil {
		c.Tools = *o.Tools
	}
	if o.ParallelTools != nil {
		c.ParallelTools = *o.ParallelTools
	}
	if o.Vision != nil {
		c.Vision = *o.Vision
	}
	if o.PromptCache != nil {
		c.PromptCache = *o.PromptCache
	}
	if o.NativeSchema != nil {
		c.NativeSchema = *o.NativeSchema
	}
	if o.Efforts != nil {
		c.Efforts = slices.Clone(o.Efforts)
	}
	if o.EffortMap != nil {
		c.EffortMap = maps.Clone(o.EffortMap)
	}
	if o.EffortBudget != nil {
		c.EffortBudget = maps.Clone(o.EffortBudget)
	}
	if o.Tokenizer != nil {
		c.Tokenizer = *o.Tokenizer
	}
	return c
}

// CapsTable 是能力表和错误规则（E.10.05 的 quirks.Table 实现它）。
type CapsTable interface {
	Lookup(protocol, upstreamModel string) Caps
	ErrorClass(protocol string, status int, code, message string) (class string, ok bool)
}

// EmitPrefix 是结构化输出用的强制工具名的前缀（E.10.01 §5.1 第 5 步）。
const EmitPrefix = "emit_"

// StructuredTool 是不支持原生结构化输出时用的强制工具：emit_<名字>，参数就是结构化结果。
func StructuredTool(o *OutputSpec) ToolSpec {
	return ToolSpec{Name: EmitPrefix + o.Name, Description: "按要求的格式输出结果（结构化输出）", Schema: o.Schema}
}
