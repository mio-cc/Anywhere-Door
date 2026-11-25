package schema

import (
	"bytes"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mio-cc/anywhere-door/gen"
)

// repoFS 是仓库的 schema/ 目录（E.03-R4）。
func repoFS() fs.FS { return os.DirFS(filepath.Join("..", "..", "schema")) }

func loadMessages(t testing.TB, log *slog.Logger) *Messages {
	t.Helper()
	m, err := LoadMessages(repoFS(), log)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDiag(t *testing.T) {
	t.Run("E0304-T1 文案齐全", func(t *testing.T) {
		m := loadMessages(t, nil)
		// C.11 §5 的级别表
		want := map[gen.DiagCode][]string{
			gen.DiagV01: {"error"}, gen.DiagV02: {"error"}, gen.DiagV03: {"error"}, gen.DiagV04: {"error"},
			gen.DiagV05: {"error"}, gen.DiagV06: {"error"}, gen.DiagV07: {"error"}, gen.DiagV08: {"error"},
			gen.DiagV09: {"error"}, gen.DiagV10: {"error"}, gen.DiagV11: {"error"}, gen.DiagV12: {"error", "warn"},
			gen.DiagV13: {"error"}, gen.DiagV14: {"error"}, gen.DiagV15: {"warn"}, gen.DiagV16: {"error"},
			gen.DiagV17: {"warn"}, gen.DiagV18: {"warn"},
		}
		for code, levels := range want {
			if got := m.Levels(string(code)); !reflect.DeepEqual(got, levels) {
				t.Errorf("%s 的级别：想要 %v，实际 %v", code, levels, got)
			}
		}
		if len(m.diags) != len(want) {
			t.Errorf("diags.zh.yaml 有 %d 个码，想要 %d 个", len(m.diags), len(want))
		}
		for _, rule := range []string{"C1", "C2", "C4", "C6", "C8", "C10"} {
			if m.Assign(rule, map[string]any{"src": "a", "dst": "b", "dst_name": "c", "producer": "d"}) == "" {
				t.Errorf("assign.zh.yaml 缺 %s", rule)
			}
		}
		if m.Assign("C99", nil) != "" {
			t.Error("没有的规则号应当返回空")
		}
	})

	t.Run("E0304-T2 排序去重", func(t *testing.T) {
		m := loadMessages(t, nil)
		ds := []gen.Diag{
			m.Diag(gen.DiagV17, gen.DiagLevelWarn, Loc{Scope: "root", Node: "B"}, map[string]any{"label": "b"}),
			m.Diag(gen.DiagV08, gen.DiagLevelError, Loc{Scope: "root", Node: "B", Port: "in"}, map[string]any{"port": "in"}),
			m.Diag(gen.DiagV05, gen.DiagLevelError, Loc{Scope: "root", Node: "A"}, map[string]any{"reason": "x"}),
			m.Diag(gen.DiagV08, gen.DiagLevelError, Loc{Scope: "C", Node: "A", Port: "in"}, map[string]any{"port": "in"}),
			m.Diag(gen.DiagV05, gen.DiagLevelError, Loc{Scope: "root", Node: "A"}, map[string]any{"reason": "y"}),
		}
		key := func(ds []gen.Diag) []string {
			var out []string
			for _, d := range ds {
				out = append(out, string(d.Level)+"/"+str(d.Scope)+"/"+ulid(d.Node)+"/"+string(d.Code)+"/"+d.Msg)
			}
			return out
		}
		a := append([]gen.Diag(nil), ds...)
		b := []gen.Diag{ds[4], ds[3], ds[2], ds[1], ds[0]}
		Sort(a)
		Sort(b)
		if !reflect.DeepEqual(key(a), key(b)) {
			t.Errorf("排序结果和输入顺序有关：\n%v\n%v", key(a), key(b))
		}
		if a[0].Code != gen.DiagV08 || str(a[0].Scope) != "C" || a[len(a)-1].Level != gen.DiagLevelWarn {
			t.Errorf("error 在前，再按 scope、node：%v", key(a))
		}
		dd := Dedup(ds)
		if len(dd) != 4 || dd[2].Msg != ds[2].Msg {
			t.Errorf("同码同位置只留第一条：%v", key(dd))
		}
		if !HasError(ds) || HasError(ds[:1]) {
			t.Error("HasError")
		}
	})

	t.Run("E0304-T3 渲染", func(t *testing.T) {
		var logs bytes.Buffer
		m := loadMessages(t, slog.New(slog.NewTextHandler(&logs, nil)))
		d := m.Diag(gen.DiagV06, gen.DiagLevelError, Loc{Scope: "root", Node: "N1", Port: "in"}, map[string]any{"src": "方案@1", "dst": "文本@1", "reason": "版本不同"})
		if d.Msg != "「方案@1」不能接到「文本@1」：版本不同" || str(d.Scope) != "root" || ulid(d.Node) != "N1" || str(d.Port) != "in" || d.Param != nil {
			t.Errorf("渲染：%+v", d)
		}
		d = m.Diag(gen.DiagV06, gen.DiagLevelError, Loc{}, map[string]any{"src": "a", "dst": "b"})
		if !strings.Contains(d.Msg, "{reason}") || d.Scope != nil || d.Node != nil || !strings.Contains(logs.String(), "reason") {
			t.Errorf("缺变量时保留 {reason} 并记 warn：%q %s", d.Msg, logs.String())
		}
		d = m.Diag(gen.DiagV17, gen.DiagLevelError, Loc{}, nil)
		if d.Msg != "V17" {
			t.Errorf("没有这个级别的文案时 msg 写诊断码：%q", d.Msg)
		}
		if got := m.Assign("C2", map[string]any{"dst_name": "上下文包", "producer": "上下文装配"}); got != "上下文包只能来自上下文装配节点" {
			t.Errorf("Assign：%q", got)
		}
		_, err := LoadMessages(fstest.MapFS{DiagsFile: {Data: []byte("V01:\n  error: [坏\n")}, AssignFile: {Data: []byte("C1: x\n")}}, nil)
		if err == nil || !strings.Contains(err.Error(), DiagsFile) || !strings.Contains(err.Error(), "第 ") {
			t.Errorf("文案文件格式错应当写出文件和行号：%v", err)
		}
		if _, err := LoadMessages(fstest.MapFS{}, nil); err == nil {
			t.Error("文案文件不存在应当报错")
		}
	})
}
