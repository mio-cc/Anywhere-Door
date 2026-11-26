package llm

import (
	"sync"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

// CodeKeysCooling：提供方的密钥都在冷却（E.10.10、E.10.09）。
const CodeKeysCooling = "keys_cooling"

// 冷却时长（E.10.10 §5）。
const (
	coolRateLimited = time.Minute
	coolAuthDenied  = 10 * time.Minute
)

// KeyRing 按轮询选密钥，跳过冷却中的（E.10.10）。并发安全；冷却只在内存里。
type KeyRing struct {
	mu   sync.Mutex
	next map[string]int       // 提供方 → 下次从第几个开始找
	cool map[string]time.Time // 提供方 + "\x00" + 密钥名 → 冷却到什么时候
}

// NewKeyRing 建空的密钥轮询表。
func NewKeyRing() *KeyRing {
	return &KeyRing{next: map[string]int{}, cool: map[string]time.Time{}}
}

func coolKey(provider, key string) string { return provider + "\x00" + key }

// Pick 从 keys 里按轮询取下一个不在冷却中的密钥名；都在冷却时 ok 为 false，wait 是最早恢复的剩余时间。
func (k *KeyRing) Pick(provider string, keys []string, now time.Time) (key string, wait time.Duration, ok bool) {
	if len(keys) == 0 {
		return "", 0, false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	start := k.next[provider] % len(keys)
	var soonest time.Time
	for i := range keys {
		j := (start + i) % len(keys)
		until, cooling := k.cool[coolKey(provider, keys[j])]
		if !cooling || !now.Before(until) {
			delete(k.cool, coolKey(provider, keys[j]))
			k.next[provider] = j + 1
			return keys[j], 0, true
		}
		if soonest.IsZero() || until.Before(soonest) {
			soonest = until
		}
	}
	return "", soonest.Sub(now), false
}

// Cool 让一个密钥冷却到 until；已在冷却时取更晚的。
func (k *KeyRing) Cool(provider, key string, until time.Time) {
	k.mu.Lock()
	defer k.mu.Unlock()
	ck := coolKey(provider, key)
	if cur, ok := k.cool[ck]; !ok || until.After(cur) {
		k.cool[ck] = until
	}
}

// CoolFor 是这类错误要让密钥冷却多久；0 表示这类错误不换密钥。
func CoolFor(e *Error) time.Duration {
	if e == nil {
		return 0
	}
	switch e.Class {
	case gen.FailureClassRateLimited:
		if e.RetryAfter > 0 {
			return e.RetryAfter
		}
		return coolRateLimited
	case gen.FailureClassAuthDenied:
		return coolAuthDenied
	}
	return 0
}
