package llm

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestKeysAndFallbacks(t *testing.T) {
	t.Run("E1001-T8 多密钥与备用提供方", func(t *testing.T) {
		srv, rec := recordingServer(t)
		sec := &testSecrets{vals: map[string]string{"a": "key-AAAA1111", "b": "key-BBBB2222"}}
		reg := NewRegistry([]Protocol{testProto{}}, nil, sec, Options{})
		ps := []ProviderConfig{
			{Name: "p", Protocol: ProtoChat, BaseURL: srv.URL, Auth: Auth{Type: AuthBearer}, Keys: []string{"a", "b"}, Fallback: []string{"x", "y", "z"}},
			{Name: "x", Protocol: ProtoChat, BaseURL: srv.URL, Auth: Auth{Type: AuthNone}},
			{Name: "y", Protocol: ProtoChat, BaseURL: srv.URL, Auth: Auth{Type: AuthNone}, Priority: 5},
			{Name: "z", Protocol: ProtoChat, BaseURL: srv.URL, Auth: Auth{Type: AuthNone}, Priority: 5},
		}
		if err := reg.SetProviders(ps); err != nil {
			t.Fatal(err)
		}
		send := func(c Client) {
			t.Helper()
			st, err := c.Complete(context.Background(), Req{Model: "m", Messages: []Message{{Role: RoleUser, Parts: []Part{{Type: PartText, Text: "hi"}}}}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Collect(st); err != nil {
				t.Fatal(err)
			}
		}
		c, err := reg.ClientWithKey("p", "m", "b")
		if err != nil {
			t.Fatal(err)
		}
		send(c)
		if got := rec.last(t).header.Get("Authorization"); got != "Bearer key-BBBB2222" {
			t.Errorf("ClientWithKey 应当用指定的密钥：%q", got)
		}
		if c, err = reg.Client("p", "m"); err != nil {
			t.Fatal(err)
		}
		send(c)
		if got := rec.last(t).header.Get("Authorization"); got != "Bearer key-AAAA1111" {
			t.Errorf("Client 应当用 keys 的第一个：%q", got)
		}
		var le *Error
		if _, err := reg.ClientWithKey("p", "m", "c"); !errors.As(err, &le) || le.Code != CodeKeyNotFound {
			t.Errorf("不在 keys 里的名字应当是 key_not_found：%v", err)
		}
		ks := reg.Keys("p")
		ks[0] = "changed"
		if got := reg.Keys("p"); !slices.Equal(got, []string{"a", "b"}) {
			t.Errorf("Keys 应当返回副本：%v", got)
		}
		if reg.Keys("x") != nil || reg.Keys("没有") != nil {
			t.Error("没写 keys 或没有这个提供方时为 nil")
		}
		if got := reg.Fallbacks("p"); !slices.Equal(got, []string{"y", "z", "x"}) {
			t.Errorf("Fallbacks 按 priority 从大到小、一样大保持原顺序：%v", got)
		}
		if reg.Fallbacks("x") != nil {
			t.Error("没写 fallback 时为 nil")
		}
	})
}
