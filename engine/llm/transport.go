package llm

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

// EnvRecord 为 1 时测试守卫放行（E.10.08 的 -record 设置它，E.10-R5、Q.06-R2）。
const EnvRecord = "ANYDOOR_LLM_RECORD"

// NewTransport 是缺省的传输层（E.10.01 §5.3）：代理按 proxy（空时用系统代理设置），连接超时 10 秒，
// 外面套测试守卫。录放（E.10.08）录制时也用它真发请求。
func NewTransport(proxy string) (http.RoundTripper, error) {
	t := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 0,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		IdleConnTimeout:       90 * time.Second,
	}
	if proxy != "" {
		u, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("llm: 代理地址 %q 不合法：%w", proxy, err)
		}
		t.Proxy = http.ProxyURL(u)
	}
	return guard{next: t}, nil
}

// guard 是测试守卫（E.10-R5）：测试二进制里、没有设 ANYDOOR_LLM_RECORD=1 时，
// 目标主机不是回环地址的请求在发出前就失败。
type guard struct{ next http.RoundTripper }

func (g guard) RoundTrip(r *http.Request) (*http.Response, error) {
	if guardBlocks(r.URL) {
		if r.Body != nil {
			_ = r.Body.Close()
		}
		return nil, guardError()
	}
	return g.next.RoundTrip(r)
}

func guardBlocks(u *url.URL) bool {
	return testing.Testing() && os.Getenv(EnvRecord) != "1" && !isLoopback(u.Hostname())
}

func guardError() *Error {
	return newError(gen.FailureClassInternal, CodeTestGuard, "测试中不允许调用真实模型")
}
