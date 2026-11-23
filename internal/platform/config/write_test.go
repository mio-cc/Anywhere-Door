package config

import (
	"os"
	"strings"
	"testing"
)

// isSubsequence 判断 want 的每一行都按顺序出现在 got 里。
func isSubsequence(want, got []string) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	return i == len(want)
}

func TestWrite(t *testing.T) {
	t.Run("B0703-T2 保留", func(t *testing.T) {
		d := openDir(t)
		orig := "# Anywhere Door 配置\nserver:\n  port: 7300\nlog:\n  level: info\n# 未知项，要原样保留\nmystery:\n  keep: 1\npackages:\n  demo_pkg:\n    greeting: hi\nnotify:\n  approval: false\n"
		writeConfig(t, d, orig)
		must(t, Write(d, map[string]any{"log.level": "debug", "notify.desktop": true}))
		b, err := os.ReadFile(d.Config())
		must(t, err)
		out := string(b)
		var keep []string
		for _, l := range strings.Split(strings.TrimSuffix(orig, "\n"), "\n") {
			if l != "  level: info" {
				keep = append(keep, l)
			}
		}
		if !isSubsequence(keep, strings.Split(out, "\n")) {
			t.Errorf("没改的行（含注释和未知项）应当逐字保留，结果：\n%s", out)
		}
		if !strings.Contains(out, "  level: debug") {
			t.Errorf("新值没写进去：\n%s", out)
		}
		c := load(t, Args{}, nil, d)
		if c.String("log.level") != "debug" || !c.Bool("notify.desktop") || c.Bool("notify.approval") || c.Int("server.port") != 7300 {
			t.Errorf("读回的值不对：\n%s", out)
		}
		if v, ok := c.Get("packages.demo_pkg.greeting"); !ok || v != "hi" {
			t.Errorf("packages 丢了：\n%s", out)
		}

		for _, bad := range []map[string]any{
			{"security.taint_guard": false},
			{"data.dir": "/x"},
			{"nope.key": 1},
			{"server.port": "abc"},
			{"sandbox.timeout_s": 99},
			{"log.level": "debug", "ui.theme.mode": "purple"},
		} {
			if err := Write(d, bad); err == nil {
				t.Errorf("Write(%v) 应当报错", bad)
			}
			if b2, _ := os.ReadFile(d.Config()); string(b2) != out {
				t.Errorf("Write(%v) 失败后文件不应当变", bad)
			}
		}

		d2 := openDir(t)
		must(t, Write(d2, map[string]any{"ui.theme.mode": "dark", "packages.demo_pkg.greeting": "hello"}))
		c2 := load(t, Args{}, nil, d2)
		if c2.String("ui.theme.mode") != "dark" {
			t.Error("文件不存在时应当新建")
		}
		if v, _ := c2.Get("packages.demo_pkg.greeting"); v != "hello" {
			t.Errorf("包配置写入：%v", v)
		}
	})
}
