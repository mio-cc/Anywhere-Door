package llm

import (
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

// RetryCfg 是重试与退避的设置（E.10.09 §5）。
type RetryCfg struct {
	Max           int           // 一个提供方上一共最多几次真实调用（换密钥不算）
	Base          time.Duration // 第一次退避
	MaxDelay      time.Duration // 退避上限
	MaxRetryAfter time.Duration // 上游的 Retry-After 超过它就不等，当作这个提供方失败
	Jitter        float64       // 抖动比例：乘 1 + Jitter × (2 × Rand() − 1)
}

// DefaultRetry 是缺省的重试设置。
var DefaultRetry = RetryCfg{Max: 3, Base: time.Second, MaxDelay: 30 * time.Second, MaxRetryAfter: 2 * time.Minute, Jitter: 0.2}

func retryable(e *Error) bool {
	return e.Class == gen.FailureClassTransient || e.Class == gen.FailureClassRateLimited
}

// delay 是第 n 次重试（从 0 起）之前要等多久；返回负数表示不等、当作失败。
func (c RetryCfg) delay(n int, e *Error, rnd func() float64) time.Duration {
	if e.Class == gen.FailureClassRateLimited && e.RetryAfter > 0 {
		if e.RetryAfter > c.MaxRetryAfter {
			return -1
		}
		return e.RetryAfter
	}
	d := c.Base
	for i := 0; i < n && d < c.MaxDelay; i++ {
		d *= 2
	}
	if d > c.MaxDelay {
		d = c.MaxDelay
	}
	if rnd != nil && c.Jitter > 0 {
		d = time.Duration(float64(d) * (1 + c.Jitter*(2*rnd()-1)))
	}
	return d
}
