package llm

import (
	"slices"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

func picks(t *testing.T, k *KeyRing, provider string, keys []string, now time.Time, n int) []string {
	t.Helper()
	var out []string
	for i := 0; i < n; i++ {
		key, _, ok := k.Pick(provider, keys, now)
		if !ok {
			t.Fatalf("第 %d 次没取到密钥", i+1)
		}
		out = append(out, key)
	}
	return out
}

func TestKeyRing(t *testing.T) {
	now := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	abc := []string{"a", "b", "c"}
	t.Run("E1010-T1 轮询", func(t *testing.T) {
		k := NewKeyRing()
		if got := picks(t, k, "p", abc, now, 4); !slices.Equal(got, []string{"a", "b", "c", "a"}) {
			t.Errorf("轮询顺序：%v", got)
		}
		if got := picks(t, k, "q", abc, now, 2); !slices.Equal(got, []string{"a", "b"}) {
			t.Errorf("两个提供方的游标互不影响：%v", got)
		}
		if _, _, ok := k.Pick("p", nil, now); ok {
			t.Error("没有密钥时取不到")
		}
	})
	t.Run("E1010-T2 冷却", func(t *testing.T) {
		k := NewKeyRing()
		k.Cool("p", "b", now.Add(time.Minute))
		if got := picks(t, k, "p", abc, now, 3); !slices.Equal(got, []string{"a", "c", "a"}) {
			t.Errorf("冷却中的 b 被跳过：%v", got)
		}
		k.Cool("p", "a", now.Add(30*time.Second))
		k.Cool("p", "c", now.Add(2*time.Minute))
		if _, wait, ok := k.Pick("p", abc, now); ok || wait != 30*time.Second {
			t.Errorf("都在冷却时取不到，wait 是最早恢复的剩余时间：%v %v", wait, ok)
		}
		if key, _, ok := k.Pick("p", abc, now.Add(31*time.Second)); !ok || key != "a" {
			t.Errorf("a 到期后恢复：%q %v", key, ok)
		}
		k.Cool("p", "c", now.Add(time.Second))
		if _, _, ok := k.Pick("p", []string{"c"}, now.Add(90*time.Second)); ok {
			t.Error("Cool 取更晚的到期时间：c 应当冷却到 2 分钟")
		}
	})
	t.Run("E1010-T3 冷却时长", func(t *testing.T) {
		for _, c := range []struct {
			e    *Error
			want time.Duration
		}{
			{&Error{Class: gen.FailureClassRateLimited, RetryAfter: 5 * time.Second}, 5 * time.Second},
			{&Error{Class: gen.FailureClassRateLimited}, time.Minute},
			{&Error{Class: gen.FailureClassAuthDenied}, 10 * time.Minute},
			{&Error{Class: gen.FailureClassTransient}, 0},
			{nil, 0},
		} {
			if got := CoolFor(c.e); got != c.want {
				t.Errorf("CoolFor(%+v) = %v，应当是 %v", c.e, got, c.want)
			}
		}
	})
}
