package datadir

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaultAndResolve(t *testing.T) {
	t.Run("B0705-T1 默认位置", func(t *testing.T) {
		cases := []struct {
			goos string
			env  map[string]string
			home string
			want string
		}{
			{"windows", map[string]string{"APPDATA": `C:\Users\mio\AppData\Roaming`}, `C:\Users\mio`, `C:\Users\mio\AppData\Roaming\AnywhereDoor`},
			{"windows", map[string]string{"APPDATA": `D:\Roam\`}, `C:\Users\mio`, `D:\Roam\AnywhereDoor`},
			{"windows", nil, `C:\Users\mio`, `C:\Users\mio\AppData\Roaming\AnywhereDoor`},
			{"darwin", nil, "/Users/mio", "/Users/mio/Library/Application Support/AnywhereDoor"},
			{"darwin", map[string]string{"XDG_DATA_HOME": "/x"}, "/Users/mio", "/Users/mio/Library/Application Support/AnywhereDoor"},
			{"linux", map[string]string{"XDG_DATA_HOME": "/data/xdg"}, "/home/mio", "/data/xdg/anydoor"},
			{"linux", nil, "/home/mio", "/home/mio/.local/share/anydoor"},
			{"linux", map[string]string{"XDG_DATA_HOME": "rel/xdg"}, "/home/mio", "/home/mio/.local/share/anydoor"},
			{"freebsd", nil, "/home/mio", "/home/mio/.local/share/anydoor"},
		}
		for _, c := range cases {
			if got := DefaultDir(c.goos, env(c.env), c.home); got != c.want {
				t.Errorf("DefaultDir(%s, %v) = %s，应当 %s", c.goos, c.env, got, c.want)
			}
		}

		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		flagDir := filepath.Join(t.TempDir(), "flag")
		envDir := filepath.Join(t.TempDir(), "env")
		t.Setenv(EnvDataDir, envDir)
		if got, _ := Resolve(flagDir, true); got != flagDir {
			t.Errorf("flag 优先：%s，应当 %s", got, flagDir)
		}
		if got, _ := Resolve("", true); got != envDir {
			t.Errorf("环境变量其次：%s，应当 %s", got, envDir)
		}
		t.Setenv(EnvDataDir, "")
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := Resolve("", true); got != filepath.Join(wd, ".anydoor-dev") {
			t.Errorf("开发模式：%s", got)
		}
		if got, _ := Resolve("", false); got != DefaultDir(runtime.GOOS, os.Getenv, home) {
			t.Errorf("默认位置：%s", got)
		}
		if got, _ := Resolve("~/ad-test", false); got != filepath.Join(home, "ad-test") {
			t.Errorf("~ 展开：%s，应当 %s", got, filepath.Join(home, "ad-test"))
		}
		if got, _ := Resolve("rel-dir", false); got != filepath.Join(wd, "rel-dir") {
			t.Errorf("相对路径转绝对：%s", got)
		}
		t.Setenv("ANYDOOR_TEST_ROOT", flagDir)
		if got, _ := ExpandPath("$ANYDOOR_TEST_ROOT/x", ""); got != filepath.Join(flagDir, "x") {
			t.Errorf("$VAR 展开：%s", got)
		}
		if got, _ := ExpandPath("sub", flagDir); got != filepath.Join(flagDir, "sub") {
			t.Errorf("按 base 转绝对：%s", got)
		}
	})
}

func TestOpen(t *testing.T) {
	t.Run("B0705-T2 创建与权限", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "AnywhereDoor")
		d, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{d.Blobs(), d.Workspaces(), d.Extensions(), d.MCP(), d.Skills(), d.Logs(), d.Tmp()} {
			if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
				t.Errorf("%s 应当是目录：%v", p, err)
			}
		}
		if ok, err := IsPrivate(d.Root); err != nil || !ok {
			t.Errorf("根目录应当只有当前用户能访问：%v %v", ok, err)
		}
		if err := WritePrivate(d.Token(), []byte("x\n")); err != nil {
			t.Fatal(err)
		}
		if ok, err := IsPrivate(d.Token()); err != nil || !ok {
			t.Errorf("WritePrivate 写的文件应当只有当前用户能访问：%v %v", ok, err)
		}
		if _, err := Open("relative/dir"); err == nil {
			t.Error("相对路径应当返回错误")
		}
		for name, got := range map[string]string{"anydoor.db": d.DB(), "anydoor.yaml": d.Config(), "port": d.Port(),
			"anydoor.lock": d.Lock(), "secrets.enc": d.Secrets(), "secrets.index.json": d.SecretsIndex()} {
			if got != filepath.Join(d.Root, name) {
				t.Errorf("%s 的路径是 %s", name, got)
			}
		}
	})
	t.Run("B0705-T3 tmp 清空", func(t *testing.T) {
		root := t.TempDir()
		d, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		must := func(err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		}
		must(os.WriteFile(filepath.Join(d.Tmp(), "a.txt"), []byte("a"), 0o600))
		must(os.MkdirAll(filepath.Join(d.Tmp(), "sub", "deep"), 0o700))
		must(os.WriteFile(filepath.Join(d.Logs(), "keep.log"), []byte("k"), 0o600))
		if _, err := Open(root); err != nil {
			t.Fatal(err)
		}
		left, err := os.ReadDir(d.Tmp())
		if err != nil || len(left) != 0 {
			t.Errorf("tmp 应当是空的：%v %v", left, err)
		}
		if _, err := os.Stat(filepath.Join(d.Logs(), "keep.log")); err != nil {
			t.Errorf("其他目录里的文件应当还在：%v", err)
		}
	})
	t.Run("B0705-T4 原子写", func(t *testing.T) {
		d, err := Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		p := d.Port()
		for _, v := range []string{"7300\n", "7301\n"} {
			if err := WritePrivate(p, []byte(v)); err != nil {
				t.Fatal(err)
			}
		}
		if b, _ := os.ReadFile(p); string(b) != "7301\n" {
			t.Errorf("覆盖后的内容是 %q", b)
		}
		entries, err := os.ReadDir(d.Root)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if strings.Contains(e.Name(), ".tmp-") {
				t.Errorf("留下了临时文件 %s", e.Name())
			}
		}
		missing := filepath.Join(d.Root, "no-such-dir", "f")
		if err := WritePrivate(missing, []byte("x")); err == nil {
			t.Error("目录不存在时应当返回错误")
		}
		if _, err := os.Stat(filepath.Join(d.Root, "no-such-dir")); err == nil {
			t.Error("不应当建出目录")
		}
	})
	t.Run("B0705-T5 覆盖正被读取的文件", func(t *testing.T) {
		d, err := Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		p := d.Config()
		if err := WritePrivate(p, []byte("a\n")); err != nil {
			t.Fatal(err)
		}
		// 别的句柄打开着目标文件，100 毫秒后才关闭：写入要重试到能改名为止
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		closed := make(chan struct{})
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = f.Close()
			close(closed)
		}()
		if err := WritePrivate(p, []byte("b\n")); err != nil {
			t.Fatalf("目标文件被短暂打开时应当重试成功：%v", err)
		}
		<-closed
		if b, _ := os.ReadFile(p); string(b) != "b\n" {
			t.Errorf("内容是 %q", b)
		}
		// 一边不停地读、一边连续写（B.07.03 热更新时写配置的情形）
		stop, done := make(chan struct{}), make(chan struct{})
		go func() {
			defer close(done)
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = os.ReadFile(p)
				}
			}
		}()
		for i := range 100 {
			if err := WritePrivate(p, []byte(strconv.Itoa(i))); err != nil {
				close(stop)
				<-done
				t.Fatalf("第 %d 次写入失败：%v", i, err)
			}
		}
		close(stop)
		<-done
		if b, _ := os.ReadFile(p); string(b) != "99" {
			t.Errorf("最后的内容是 %q", b)
		}
		// 退避间隔：1ms 起、每次翻倍、最长 100ms 一次（B.07.05 §5）
		w, waits := time.Millisecond, []time.Duration{time.Millisecond}
		for range 10 {
			w = nextWait(w)
			waits = append(waits, w)
		}
		want := []time.Duration{1, 2, 4, 8, 16, 32, 64, 100, 100, 100, 100}
		for i := range want {
			if waits[i] != want[i]*time.Millisecond {
				t.Errorf("第 %d 次等待是 %v，应当是 %v（全部：%v）", i+1, waits[i], want[i]*time.Millisecond, waits)
				break
			}
		}
		if runtime.GOOS == "windows" {
			const limit = 300 * time.Millisecond
			old := renameRetryFor
			renameRetryFor = limit
			defer func() { renameRetryFor = old }()
			g, err := os.Open(p)
			if err != nil {
				t.Fatal(err)
			}
			began := time.Now()
			err = WritePrivate(p, []byte("c\n"))
			took := time.Since(began)
			_ = g.Close()
			if err == nil {
				t.Error("目标一直被打开时应当在重试时限后返回错误")
			}
			// 一直重试到时限，等待不越过时限：用时在 [时限, 时限 + 500ms) 里（500ms 是给最后一次改名和计时误差的余量）
			if took < limit || took >= limit+500*time.Millisecond {
				t.Errorf("重试用了 %v，应当在 [%v, %v) 里", took, limit, limit+500*time.Millisecond)
			}
		}
		entries, err := os.ReadDir(d.Root)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if strings.Contains(e.Name(), ".tmp-") {
				t.Errorf("留下了临时文件 %s", e.Name())
			}
		}
	})
}
