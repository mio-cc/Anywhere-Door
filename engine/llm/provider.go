package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

// ProviderConfig 是一个模型提供方：按一种接口协议接入的一个服务（相当于 NewAPI 里的"渠道"，E.10.01 §4.3）。
// 资源节点的模型引用 {provider, model} 里的 provider 是 Name，model 是 Models[].ID。
type ProviderConfig struct {
	Name     string            `json:"name"`            // ^[a-z0-9][a-z0-9_-]{0,39}$，全局唯一
	Protocol string            `json:"protocol"`        // chat_completions | responses | anthropic_messages
	BaseURL  string            `json:"base_url"`        // 如 https://api.mistral.ai/v1；请求地址 = BaseURL + 路径
	Paths    map[string]string `json:"paths,omitempty"` // 覆盖协议的默认路径：complete、models
	Auth     Auth              `json:"auth"`
	Headers  map[string]string `json:"headers,omitempty"` // 额外请求头；值里可以写 {{secret:名}}
	Models   []ModelEntry      `json:"models,omitempty"`  // 空：任何模型名都原样发给上游
	Override []BodyOp          `json:"override,omitempty"`
	Caps     *CapsOverride     `json:"caps,omitempty"`      // 覆盖 quirks 表（对这个提供方的所有模型）
	TimeoutS int               `json:"timeout_s,omitempty"` // 整个请求的时限（秒），缺省 120
	Proxy    string            `json:"proxy,omitempty"`     // http(s):// 代理；空用系统代理设置
	Vendor   string            `json:"vendor,omitempty"`    // 从哪个常用厂商填的（E.10.05），只用于显示

	// 多密钥轮询、优先级、故障转移（P.09 K250、K268；E.10.10、E.10.09）：写了 keys 时只用 keys；fallback 是别的已有提供方名；
	// priority 决定被列在别人 fallback 里时的先后（大的先）；weight 还不生效，写了记 warn
	Keys     []string `json:"keys,omitempty"`
	Priority int      `json:"priority,omitempty"`
	Weight   int      `json:"weight,omitempty"`
	Fallback []string `json:"fallback,omitempty"`
}

// Auth 是提供方的鉴权方式。
type Auth struct {
	Type   string `json:"type"`             // bearer | header | query | none；缺省按协议
	Name   string `json:"name,omitempty"`   // header / query 时的请求头名或参数名
	Secret string `json:"secret,omitempty"` // 密钥名（B.07.02），不是明文；none 时为空
}

// Auth.Type 的取值。
const (
	AuthBearer = "bearer"
	AuthHeader = "header"
	AuthQuery  = "query"
	AuthNone   = "none"
)

// ModelEntry 是提供方的一个模型。
type ModelEntry struct {
	ID       string        `json:"id"`                 // 对上层的名字
	Upstream string        `json:"upstream,omitempty"` // 实际发给上游的名字；空 = ID
	Caps     *CapsOverride `json:"caps,omitempty"`     // 只对这个模型
}

// UpstreamName 是实际发给上游的模型名。
func (m ModelEntry) UpstreamName() string {
	if m.Upstream != "" {
		return m.Upstream
	}
	return m.ID
}

// BodyOp 是一条请求体覆盖（E.10.01 §5.2）。
type BodyOp struct {
	Op     string          `json:"op"`               // set | delete | rename
	Path   string          `json:"path"`             // 点分路径，数组下标写数字
	To     string          `json:"to,omitempty"`     // rename 的新路径
	Value  json.RawMessage `json:"value,omitempty"`  // set 的值（任意 JSON）
	Models []string        `json:"models,omitempty"` // 只对这些上游模型生效（可以用 *）；空 = 全部
}

// DefaultTimeout 是 TimeoutS 不写时整个请求的时限。
const DefaultTimeout = 120 * time.Second

// Timeout 是整个请求的时限。
func (p ProviderConfig) Timeout() time.Duration {
	if p.TimeoutS > 0 {
		return time.Duration(p.TimeoutS) * time.Second
	}
	return DefaultTimeout
}

var (
	providerNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)
	headerNameRe   = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
	pathKeys       = []string{"complete", "models"}
)

// ParseProviders 解析并校验提供方配置（JSON 数组；配置文件里的 YAML 由调用方先转成 JSON）。
// 第二个返回值是 warn；配置错误时返回第一处错误，写明是哪个提供方、哪个字段（E.10.01 §7）。
func ParseProviders(raw json.RawMessage) ([]ProviderConfig, []string, error) {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || string(t) == "null" {
		return nil, nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(t))
	dec.DisallowUnknownFields()
	var ps []ProviderConfig
	if err := dec.Decode(&ps); err != nil {
		return nil, nil, fmt.Errorf("llm: 模型提供方配置的格式不对：%w", err)
	}
	warns, err := validateProviders(ps)
	if err != nil {
		return nil, nil, err
	}
	return ps, warns, nil
}

// validateProviders 校验一组提供方配置，返回 warn。
func validateProviders(ps []ProviderConfig) ([]string, error) {
	var warns []string
	seen := map[string]int{}
	for i, p := range ps {
		at := func(field, msg string) error {
			return fmt.Errorf("llm: 模型提供方 #%d（%s）的 %s：%s", i+1, p.Name, field, msg)
		}
		warn := func(msg string) { warns = append(warns, fmt.Sprintf("模型提供方 %s：%s", p.Name, msg)) }
		if !providerNameRe.MatchString(p.Name) {
			return nil, at("name", "只能用小写字母、数字、_、-，以字母或数字开头，最长 40 个字符")
		}
		if j, ok := seen[p.Name]; ok {
			return nil, at("name", fmt.Sprintf("和第 %d 个提供方重名", j+1))
		}
		seen[p.Name] = i
		if !slices.Contains(KnownProtocols, p.Protocol) {
			return nil, at("protocol", fmt.Sprintf("不认识的协议 %q（可以用 %s）", p.Protocol, strings.Join(KnownProtocols, "、")))
		}
		u, err := url.Parse(p.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, at("base_url", fmt.Sprintf("%q 不是 http:// 或 https:// 开头的绝对地址", p.BaseURL))
		}
		for k, v := range p.Paths {
			if !slices.Contains(pathKeys, k) {
				return nil, at("paths", fmt.Sprintf("不认识的路径名 %q（可以用 complete、models）", k))
			}
			if !strings.HasPrefix(v, "/") {
				return nil, at("paths."+k, fmt.Sprintf("%q 要以 / 开头", v))
			}
		}
		auth := p.Auth
		if auth.Secret == "" && len(p.Keys) > 0 && auth.Type != AuthNone {
			auth.Secret = p.Keys[0]
		}
		if err := validateAuth(auth); err != nil {
			return nil, at("auth", err.Error())
		}
		for k := range p.Headers {
			if !headerNameRe.MatchString(k) {
				return nil, at("headers", fmt.Sprintf("请求头名 %q 不合法", k))
			}
		}
		ids := map[string]bool{}
		for j, m := range p.Models {
			if m.ID == "" {
				return nil, at(fmt.Sprintf("models[%d].id", j), "不能为空")
			}
			if ids[m.ID] {
				return nil, at(fmt.Sprintf("models[%d].id", j), fmt.Sprintf("模型 id %q 重名", m.ID))
			}
			ids[m.ID] = true
		}
		for j, op := range p.Override {
			if err := validateOp(op); err != nil {
				return nil, at(fmt.Sprintf("override[%d]", j), err.Error())
			}
		}
		if p.TimeoutS < 0 {
			return nil, at("timeout_s", "不能是负数")
		}
		if p.Proxy != "" {
			pu, err := url.Parse(p.Proxy)
			if err != nil || (pu.Scheme != "http" && pu.Scheme != "https") || pu.Host == "" {
				return nil, at("proxy", fmt.Sprintf("%q 不是 http:// 或 https:// 开头的代理地址", p.Proxy))
			}
		}
		keys := map[string]bool{}
		for j, k := range p.Keys {
			if k == "" {
				return nil, at(fmt.Sprintf("keys[%d]", j), "不能为空")
			}
			if keys[k] {
				return nil, at(fmt.Sprintf("keys[%d]", j), fmt.Sprintf("密钥名 %q 重复", k))
			}
			keys[k] = true
		}
		if len(p.Keys) > 0 && p.Auth.Secret != "" {
			warn("auth.secret 和 keys 都写了：keys 生效，auth.secret 不再使用")
		}
		if p.Weight != 0 {
			warn("weight（权重）还不生效：以后有模型分组时用（P.09 K268）")
		}
		if u.Scheme == "http" && p.Auth.Type != AuthNone && !isLoopback(u.Hostname()) {
			warn("base_url 是 http:// 的远程地址，密钥会以明文发送")
		}
	}
	for i, p := range ps {
		dup := map[string]bool{}
		for j, f := range p.Fallback {
			at := fmt.Sprintf("llm: 模型提供方 #%d（%s）的 fallback[%d]：", i+1, p.Name, j)
			if f == p.Name {
				return nil, fmt.Errorf("%s不能指向自己", at)
			}
			if dup[f] {
				return nil, fmt.Errorf("%s%q 重复", at, f)
			}
			if _, ok := seen[f]; !ok {
				return nil, fmt.Errorf("%s没有名为 %q 的提供方", at, f)
			}
			dup[f] = true
		}
	}
	return warns, nil
}

func validateAuth(a Auth) error {
	switch a.Type {
	case "", AuthBearer:
	case AuthHeader, AuthQuery:
		if a.Name == "" {
			return fmt.Errorf("type 为 %s 时要写 name（请求头名或参数名）", a.Type)
		}
	case AuthNone:
		if a.Secret != "" {
			return fmt.Errorf("type 为 none 时不要写 secret")
		}
		return nil
	default:
		return fmt.Errorf("不认识的 type %q（可以用 bearer、header、query、none）", a.Type)
	}
	if a.Secret == "" {
		return fmt.Errorf("需要密钥：secret 写密钥名（不是明文）；不需要鉴权时 type 写 none")
	}
	return nil
}

func validateOp(op BodyOp) error {
	switch op.Op {
	case OpSet:
		if len(op.Value) == 0 || !json.Valid(op.Value) {
			return fmt.Errorf("set 要写 value（任意 JSON）")
		}
	case OpDelete:
	case OpRename:
		if op.To == "" {
			return fmt.Errorf("rename 要写 to")
		}
	default:
		return fmt.Errorf("不认识的 op %q（可以用 set、delete、rename）", op.Op)
	}
	if !dottedPath(op.Path) {
		return fmt.Errorf("path %q 不是点分路径", op.Path)
	}
	if op.Op == OpRename && !dottedPath(op.To) {
		return fmt.Errorf("to %q 不是点分路径", op.To)
	}
	for _, m := range op.Models {
		if _, err := path.Match(m, ""); err != nil {
			return fmt.Errorf("models 里的 %q 不是合法的通配", m)
		}
	}
	return nil
}

// dottedPath：非空、每一段都不为空的点分路径（如 stream_options.include_usage、messages.0）。
func dottedPath(p string) bool {
	if p == "" {
		return false
	}
	for _, s := range strings.Split(p, ".") {
		if s == "" {
			return false
		}
	}
	return true
}

// isLoopback：127.0.0.1、::1、localhost（E.10.01 §5.3）。
func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
