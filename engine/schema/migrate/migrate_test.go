package migrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestApply(t *testing.T) {
	t.Run("E0301-T9 图文档迁移", func(t *testing.T) {
		in := []byte(`{"id":"G","schema_version":"1.0","version":2,"root":{"nodes":[],"edges":[]}}`)
		keep := append([]byte(nil), in...)

		// C12-T1：迁移链为空时 1.0 原样通过
		out, from, err := Apply(in, Chain)
		if err != nil || from != "1.0" || !bytes.Equal(out, in) {
			t.Fatalf("空链：%s %s %v", out, from, err)
		}

		called := 0
		chain := []Step{{From: "1.0", To: "1.1", Fn: func(d map[string]any) (map[string]any, error) {
			called++
			d["title"] = "升级过"
			return d, nil
		}}}
		out, from, err = Apply(in, chain)
		if err != nil || from != "1.0" || called != 1 {
			t.Fatalf("1.0 → 1.1：%v，调用 %d 次", err, called)
		}
		var doc map[string]any
		if err := json.Unmarshal(out, &doc); err != nil || doc["schema_version"] != "1.1" || doc["title"] != "升级过" {
			t.Errorf("结果：%s", out)
		}
		if !bytes.Equal(in, keep) {
			t.Error("输入的字节被改了")
		}
		// 已经是 1.1：链上没有 From=1.1 的步骤，原样返回；1.1 不算太新
		if out2, _, err := Apply(out, chain); err != nil || !bytes.Equal(out2, out) {
			t.Errorf("已是最新：%v", err)
		}

		// C12-T2：9.0 被拒绝
		_, from, err = Apply([]byte(`{"schema_version":"9.0"}`), Chain)
		if !errors.Is(err, ErrTooNew) || from != "9.0" || err.Error() != "这张图由更新版本的 Anywhere Door 创建，请升级" {
			t.Errorf("9.0：%v", err)
		}
		if _, _, err := Apply([]byte(`{"schema_version":"1.1"}`), Chain); !errors.Is(err, ErrTooNew) {
			t.Errorf("空链时 1.1 也比 Current 新：%v", err)
		}
		for _, bad := range []string{`{}`, `{"schema_version":"一"}`, `{"schema_version":1}`} {
			if _, _, err := Apply([]byte(bad), Chain); !errors.Is(err, ErrNoVersion) {
				t.Errorf("%s：%v", bad, err)
			}
		}
		if _, _, err := Apply([]byte(`[1]`), Chain); err == nil {
			t.Error("不是对象应当报错")
		}
		fail := []Step{{From: "1.0", To: "1.1", Fn: func(map[string]any) (map[string]any, error) { return nil, errors.New("坏了") }}}
		if _, _, err := Apply(in, fail); err == nil {
			t.Error("迁移函数出错要返回错误")
		}
	})
}
