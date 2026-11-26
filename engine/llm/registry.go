package llm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

// Protocol 是一种接口协议的编码和解码（E.10.01 §4.4）。协议代码不知道自己连的是哪一家（E.10-R2）。
type Protocol interface {
	Name() string                      // chat_completions 等
	DefaultPaths() map[string]string   // complete、models
	DefaultAuth() Auth                 // Type 和 Name；Secret 来自提供方配置
	DefaultHeaders() map[string]string // 如 anthropic_messages 的 anthropic-version
	// Encode 把统一请求编码成协议的请求体（流式）；之后再套提供方的 Override。
	Encode(r Req, upstream string, c Caps) (body map[string]any, err error)
	// Decode 把流式响应（SSE）解码成 Delta。协议只需要给出文本、思考、工具调用的片段，
	// 结束时给 Done 和带 Finish、Usage、Model 的 Resp；拼接、结构化输出、Raw 由本包补齐。
	Decode(body io.Reader, c Caps) Stream
	DecodeError(status int, body []byte) (code, message string) // 从错误响应里取上游的错误码和说明
	DecodeModels(body []byte) ([]string, error)
}

// SecretResolver 按密钥名取密钥（组装处用 secret.Store 包一层实现它；llm 不 import 平台层，INV-19）。
// 密钥库锁定时返回包着 ErrSecretLocked 的错误。
type SecretResolver interface {
	Resolve(ctx context.Context, name string) (string, error)
}

// Options 是 Registry 的可选项。
type Options struct {
	Transport http.RoundTripper // nil：按提供方的 Proxy 建（NewTransport）；测试里换成录放（E.10.08）
	Logger    *slog.Logger
}

// Registry 是模型提供方注册表（E.10.01 §4.4），并发安全。
type Registry struct {
	protocols map[string]Protocol
	caps      CapsTable
	secrets   SecretResolver
	opt       Options
	log       *slog.Logger

	mu        sync.RWMutex
	providers []ProviderConfig
	byName    map[string]*provider
}

type provider struct {
	cfg   ProviderConfig
	proto Protocol
	http  *http.Client
}

// NewRegistry 建注册表。协议在组装处显式传进来（不用 init 全局注册，E-R3）。
func NewRegistry(protocols []Protocol, caps CapsTable, secrets SecretResolver, o Options) *Registry {
	r := &Registry{protocols: map[string]Protocol{}, caps: caps, secrets: secrets, opt: o, log: o.Logger, byName: map[string]*provider{}}
	if r.log == nil {
		r.log = slog.New(slog.DiscardHandler)
	}
	for _, p := range protocols {
		r.protocols[p.Name()] = p
	}
	return r
}

// SetProviders 整体替换提供方（配置热更新时）。不调用 SecretResolver。
func (r *Registry) SetProviders(ps []ProviderConfig) error {
	warns, err := validateProviders(ps)
	if err != nil {
		return err
	}
	by := make(map[string]*provider, len(ps))
	for _, p := range ps {
		proto, ok := r.protocols[p.Protocol]
		if !ok {
			return fmt.Errorf("llm: 模型提供方 %s 的 protocol：协议 %q 没有注册", p.Name, p.Protocol)
		}
		rt := r.opt.Transport
		if rt == nil {
			if rt, err = NewTransport(p.Proxy); err != nil {
				return fmt.Errorf("llm: 模型提供方 %s 的 proxy：%w", p.Name, err)
			}
		}
		by[p.Name] = &provider{cfg: p, proto: proto, http: &http.Client{Transport: rt}}
	}
	for _, w := range warns {
		r.log.Warn(w)
	}
	r.mu.Lock()
	r.providers = append([]ProviderConfig(nil), ps...)
	r.byName = by
	r.mu.Unlock()
	return nil
}

// Providers 返回当前的提供方配置。
func (r *Registry) Providers() []ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]ProviderConfig(nil), r.providers...)
}

func (r *Registry) lookup(name string) (*provider, error) {
	r.mu.RLock()
	p := r.byName[name]
	r.mu.RUnlock()
	if p == nil {
		return nil, &Error{Class: gen.FailureClassUserInput, Provider: name, Code: "provider_not_found", Message: fmt.Sprintf("没有名为 %s 的模型提供方", name)}
	}
	return p, nil
}

// Client 把资源节点的模型引用 {provider, model} 换成可调用的 Client（E.10.01 §5.1 第 1 步）。
func (r *Registry) Client(providerName, model string) (Client, error) {
	return r.clientFor(providerName, model, "")
}

// CodeKeyNotFound：ClientWithKey 给的密钥名不在提供方的 keys 里。
const CodeKeyNotFound = "key_not_found"

// ClientWithKey 同 Client，鉴权用指定的密钥名（必须在提供方的 keys 里；换密钥在 E.10.10、E.10.09）。
func (r *Registry) ClientWithKey(providerName, model, key string) (Client, error) {
	p, err := r.lookup(providerName)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(p.cfg.Keys, key) {
		return nil, &Error{Class: gen.FailureClassUserInput, Provider: p.cfg.Name, Code: CodeKeyNotFound, Message: fmt.Sprintf("提供方 %s 的 keys 里没有密钥 %s", p.cfg.Name, key)}
	}
	return r.clientFor(providerName, model, key)
}

// Keys 是提供方的 keys（副本）；没写或没有这个提供方时为 nil。
func (r *Registry) Keys(providerName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p := r.byName[providerName]; p != nil {
		return slices.Clone(p.cfg.Keys)
	}
	return nil
}

// Fallbacks 是提供方的 fallback，按目标提供方的 priority 从大到小（一样大保持原顺序）。
func (r *Registry) Fallbacks(providerName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p := r.byName[providerName]
	if p == nil || len(p.cfg.Fallback) == 0 {
		return nil
	}
	prio := func(name string) int {
		if q := r.byName[name]; q != nil {
			return q.cfg.Priority
		}
		return 0
	}
	out := slices.Clone(p.cfg.Fallback)
	slices.SortStableFunc(out, func(a, b string) int { return prio(b) - prio(a) })
	return out
}

func (r *Registry) clientFor(providerName, model, key string) (Client, error) {
	p, err := r.lookup(providerName)
	if err != nil {
		return nil, err
	}
	entry := ModelEntry{ID: model}
	if len(p.cfg.Models) > 0 {
		found := false
		for _, m := range p.cfg.Models {
			if m.ID == model {
				entry, found = m, true
				break
			}
		}
		if !found {
			return nil, &Error{Class: gen.FailureClassUserInput, Provider: p.cfg.Name, Code: "model_not_found", Message: fmt.Sprintf("提供方 %s 没有模型 %s", p.cfg.Name, model)}
		}
	}
	upstream := entry.UpstreamName()
	var caps Caps
	if r.caps != nil {
		caps = r.caps.Lookup(p.proto.Name(), upstream)
	}
	caps = caps.Apply(p.cfg.Caps).Apply(entry.Caps)
	return &client{r: r, p: p, upstream: upstream, caps: caps, key: key}, nil
}

// ListModels 取上游的模型列表（"测试连接"用）。
func (r *Registry) ListModels(ctx context.Context, providerName string) ([]string, error) {
	p, err := r.lookup(providerName)
	if err != nil {
		return nil, err
	}
	c := &client{r: r, p: p}
	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout())
	defer cancel()
	req, secrets, err := c.newRequest(ctx, http.MethodGet, "models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return nil, c.transportError(ctx, err, secrets)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil, c.statusError(resp, secrets)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, c.transportError(ctx, err, secrets)
	}
	ms, err := p.proto.DecodeModels(b)
	if err != nil {
		return nil, c.fill(&Error{Class: gen.FailureClassInternal, Code: "bad_models", Message: "模型列表的格式不对：" + err.Error()}, secrets)
	}
	return ms, nil
}

// client 是一个提供方的一个模型。
type client struct {
	r        *Registry
	p        *provider
	upstream string
	caps     Caps
	key      string // ClientWithKey 指定的密钥名；空：keys 的第一个，没写 keys 时用 auth.secret
}

func (c *client) Caps() Caps { return c.caps }

// Complete 发一次流式调用（E.10.01 §5.1 第 2–5 步）。
func (c *client) Complete(ctx context.Context, req Req) (Stream, error) {
	if req.Output != nil && !c.caps.NativeSchema && !c.caps.Tools {
		return nil, c.fill(newError(gen.FailureClassUserInput, "unsupported_output", "这个模型不支持结构化输出"), nil)
	}
	if _, warn := ResolveEffort(req.Reasoning, c.caps); warn != "" {
		c.r.log.Warn(warn, "provider", c.p.cfg.Name, "model", c.upstream)
	}
	enc, err := c.p.proto.Encode(req, c.upstream, c.caps)
	if err != nil {
		return nil, c.fill(newError(gen.FailureClassUserInput, "encode", err.Error()), nil)
	}
	body, err := c.buildBody(enc)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.p.cfg.Timeout())
	httpReq, secrets, err := c.newRequest(ctx, http.MethodPost, "complete", body)
	if err != nil {
		cancel()
		return nil, err
	}
	resp, err := c.p.http.Do(httpReq)
	if err != nil {
		e := c.transportError(ctx, err, secrets)
		cancel()
		return nil, e
	}
	if resp.StatusCode/100 != 2 {
		e := c.statusError(resp, secrets)
		_ = resp.Body.Close()
		cancel()
		return nil, e
	}
	t := newTap(resp.Body)
	return &stream{c: c, ctx: ctx, cancel: cancel, body: resp.Body, tap: t, inner: c.p.proto.Decode(t, c.caps), out: req.Output, secrets: secrets}, nil
}

// buildBody：规范化 → 按顺序执行 Override → 序列化（E.10.01 §5.1 第 2 步、§5.2）。
func (c *client) buildBody(enc map[string]any) ([]byte, error) {
	v, err := normalize(enc)
	if err != nil {
		return nil, c.fill(newError(gen.FailureClassInternal, "encode", "请求体序列化失败："+err.Error()), nil)
	}
	v, err = applyOverrides(v, c.p.cfg.Override, c.upstream)
	if err != nil {
		return nil, c.fill(newError(gen.FailureClassUserInput, "override", err.Error()), nil)
	}
	b, err := marshalBody(v)
	if err != nil {
		return nil, c.fill(newError(gen.FailureClassInternal, "encode", "请求体序列化失败："+err.Error()), nil)
	}
	return b, nil
}

// secretRefRe 匹配请求头值里的密钥引用（双花括号里写 secret、冒号、密钥名）。
var secretRefRe = regexp.MustCompile(`\{\{\s*secret\s*[:]\s*([^{}\s]+)\s*\}\}`)

// newRequest 组装请求（E.10.01 §5.1 第 3 步）。返回这次用到的密钥原文（脱敏用）。
// 密钥在这一刻才用 SecretResolver 取（E.10-R4）。
func (c *client) newRequest(ctx context.Context, method, pathKey string, body []byte) (*http.Request, []string, error) {
	cfg, proto := c.p.cfg, c.p.proto
	p := proto.DefaultPaths()[pathKey]
	if v, ok := cfg.Paths[pathKey]; ok {
		p = v
	}
	u := strings.TrimRight(cfg.BaseURL, "/") + p
	var secrets []string
	resolve := func(name string) (string, error) {
		if c.r.secrets == nil {
			return "", c.secretError(errors.New("没有配置密钥库"), name)
		}
		v, err := c.r.secrets.Resolve(ctx, name)
		if err != nil {
			return "", c.secretError(err, name)
		}
		secrets = append(secrets, v)
		return v, nil
	}
	headers := map[string]string{}
	for k, v := range cfg.Headers {
		var rerr error
		v = secretRefRe.ReplaceAllStringFunc(v, func(m string) string {
			if rerr != nil {
				return m
			}
			val, err := resolve(secretRefRe.FindStringSubmatch(m)[1])
			if err != nil {
				rerr = err
			}
			return val
		})
		if rerr != nil {
			return nil, nil, rerr
		}
		headers[k] = v
	}
	auth := cfg.Auth
	if auth.Type == "" {
		d := proto.DefaultAuth()
		d.Secret = auth.Secret
		if auth.Name != "" {
			d.Name = auth.Name
		}
		auth = d
	}
	if c.key != "" {
		auth.Secret = c.key
	} else if len(cfg.Keys) > 0 {
		auth.Secret = cfg.Keys[0]
	}
	var key string
	if auth.Type != AuthNone {
		v, err := resolve(auth.Secret)
		if err != nil {
			return nil, nil, err
		}
		key = v
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, nil, c.fill(newError(gen.FailureClassUserInput, "bad_url", "请求地址不合法："+err.Error()), secrets)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	for k, v := range proto.DefaultHeaders() {
		req.Header.Set(k, v)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	switch auth.Type {
	case AuthBearer:
		req.Header.Set("Authorization", "Bearer "+key)
	case AuthHeader:
		req.Header.Set(auth.Name, key)
	case AuthQuery:
		q := req.URL.Query()
		q.Set(auth.Name, key)
		req.URL.RawQuery = q.Encode()
	}
	return req, secrets, nil
}

func (c *client) secretError(err error, name string) *Error {
	if errors.Is(err, ErrSecretLocked) {
		return c.fill(newError(gen.FailureClassAuthDenied, CodeSecretLocked, "密钥库已锁定，请先解锁"), nil)
	}
	return c.fill(newError(gen.FailureClassAuthDenied, CodeSecretMissing, fmt.Sprintf("密钥 %s 不存在", name)), nil)
}

// transportError 把发请求、读响应时的错误变成 *Error（§5.5：网络错误、超时 → transient）。
func (c *client) transportError(ctx context.Context, err error, secrets []string) *Error {
	var le *Error
	if errors.As(err, &le) {
		cp := *le
		return c.fill(&cp, secrets)
	}
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
		return c.fill(newError(gen.FailureClassTransient, CodeTimeout, fmt.Sprintf("请求超时（%s）", c.p.cfg.Timeout())), secrets)
	case errors.Is(err, context.Canceled):
		return c.fill(newError(gen.FailureClassCancelled, "cancelled", "调用已取消"), secrets)
	}
	return c.fill(newError(gen.FailureClassTransient, CodeNetwork, "网络错误："+err.Error()), secrets)
}

// statusError 读最多 64 KB 的错误响应，按 §5.5 分类。
func (c *client) statusError(resp *http.Response, secrets []string) *Error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	code, msg := c.p.proto.DecodeError(resp.StatusCode, b)
	if msg == "" {
		msg = strings.TrimSpace(string(b))
	}
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	e := &Error{Status: resp.StatusCode, Code: code, Message: msg, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())}
	return c.fill(e, secrets)
}

// fill 补上提供方名、分类（Class 为空时），脱敏并截断说明。
func (c *client) fill(e *Error, secrets []string) *Error {
	e.Provider = c.p.cfg.Name
	if e.Class == "" {
		e.Class = classify(c.r.caps, c.p.proto.Name(), e.Status, e.Code, e.Message)
	}
	e.Message = truncate(Redact(e.Message, secrets...), maxMessage)
	return e
}
