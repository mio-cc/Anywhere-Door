package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"debug/buildinfo"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	r, err := filepath.Abs(filepath.Join("..", ".."))
	must(t, err)
	return r
}

func TestTargets(t *testing.T) {
	t.Run("O03-T1 目标", func(t *testing.T) {
		ts := Targets()
		if len(ts) != 6 {
			t.Fatalf("应当正好六个平台，是 %d 个", len(ts))
		}
		want := map[string]string{
			"windows/amd64": ".zip", "windows/arm64": ".zip",
			"darwin/arm64": ".tar.gz", "darwin/amd64": ".tar.gz",
			"linux/amd64": ".tar.gz", "linux/arm64": ".tar.gz",
		}
		for _, x := range ts {
			if want[x.String()] != x.Ext() {
				t.Errorf("%s 的归档应当是 %q，是 %q", x, want[x.String()], x.Ext())
			}
			delete(want, x.String())
		}
		if len(want) != 0 {
			t.Errorf("缺少平台：%v", want)
		}
		if (Target{"windows", "amd64"}).Exe("anydoor") != "anydoor.exe" || (Target{"linux", "amd64"}).Exe("anydoor") != "anydoor" {
			t.Error("程序文件名不对")
		}
	})
}

// fakeRepo 是 Archive 用的假仓库根和假的两个程序。
func fakeRepo(t *testing.T, targets ...Target) (repo, dist string) {
	t.Helper()
	repo, dist = t.TempDir(), t.TempDir()
	must(t, os.WriteFile(filepath.Join(repo, "CHANGELOG.md"),
		[]byte("# 变更日志\n\n## [未发布]\n### 新增\n- 第一个功能\n\n## [0.0.1]\n- 旧的\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(repo, "LICENSE"), []byte("license text\n"), 0o644))
	for _, x := range targets {
		dir := filepath.Join(dist, "tmp", x.OS+"-"+x.Arch)
		must(t, os.MkdirAll(dir, 0o755))
		for _, p := range []string{"anydoor", "anydoorctl"} {
			must(t, os.WriteFile(filepath.Join(dir, x.Exe(p)), []byte("binary of "+p+" "+x.String()), 0o644))
		}
	}
	return repo, dist
}

type member struct {
	name string
	mode os.FileMode
	mod  time.Time
	data string
}

func readTarGz(t *testing.T, b []byte) []member {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(b))
	must(t, err)
	tr := tar.NewReader(gz)
	var out []member
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out
		}
		must(t, err)
		d, err := io.ReadAll(tr)
		must(t, err)
		out = append(out, member{h.Name, os.FileMode(h.Mode).Perm(), h.ModTime, string(d)})
	}
}

func readZip(t *testing.T, b []byte) []member {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	must(t, err)
	var out []member
	for _, f := range zr.File {
		rc, err := f.Open()
		must(t, err)
		d, err := io.ReadAll(rc)
		must(t, err)
		_ = rc.Close()
		out = append(out, member{f.Name, f.Mode().Perm(), f.Modified, string(d)})
	}
	return out
}

func TestArchive(t *testing.T) {
	t.Run("O03-T2 归档可复现", func(t *testing.T) {
		lin, win := Target{"linux", "amd64"}, Target{"windows", "amd64"}
		repo, dist := fakeRepo(t, lin, win)
		old := root
		root = repo
		defer func() { root = old }()
		mtime := time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC)
		for _, x := range []Target{lin, win} {
			p1, err := Archive(dist, x, "0.1.0-preview", mtime)
			must(t, err)
			b1, err := os.ReadFile(p1)
			must(t, err)
			must(t, os.Remove(p1))
			p2, err := Archive(dist, x, "0.1.0-preview", mtime)
			must(t, err)
			b2, err := os.ReadFile(p2)
			must(t, err)
			if p1 != p2 || !bytes.Equal(b1, b2) {
				t.Errorf("%s：同样的输入打包两次应当字节相同", x)
			}
			top := "anydoor-0.1.0-preview-" + x.OS + "-" + x.Arch
			if filepath.Base(p1) != top+x.Ext() {
				t.Errorf("%s：归档名是 %s", x, filepath.Base(p1))
			}
			var ms []member
			if x.OS == "windows" {
				ms = readZip(t, b1)
			} else {
				ms = readTarGz(t, b1)
			}
			var names []string
			for _, m := range ms {
				names = append(names, m.name)
				if !m.mod.Equal(mtime) {
					t.Errorf("%s：%s 的修改时间是 %v，应当是提交时间 %v", x, m.name, m.mod, mtime)
				}
				wantMode := os.FileMode(0o644)
				if strings.HasSuffix(m.name, "/") || strings.Contains(m.name, "/anydoor") {
					wantMode = 0o755
				}
				if m.mode != wantMode {
					t.Errorf("%s：%s 的权限是 %v，应当是 %v", x, m.name, m.mode, wantMode)
				}
			}
			want := []string{top + "/", top + "/CHANGELOG.md", top + "/LICENSE", top + "/README.txt", top + "/" + x.Exe("anydoor"), top + "/" + x.Exe("anydoorctl")}
			if !sort.StringsAreSorted(names[1:]) || strings.Join(names, ",") != strings.Join(want, ",") {
				t.Errorf("%s：归档里的文件是 %v，应当是按名字排序的 %v（没有 NOTICE 时只警告）", x, names, want)
			}
			for _, m := range ms {
				switch {
				case strings.HasSuffix(m.name, "/CHANGELOG.md"):
					if !strings.Contains(m.data, "第一个功能") || strings.Contains(m.data, "旧的") {
						t.Errorf("%s：CHANGELOG.md 应当只有本版本（这里是\"未发布\"）那一节：%q", x, m.data)
					}
				case strings.HasSuffix(m.name, "/README.txt"):
					if !strings.Contains(m.data, "0.1.0-preview") || !strings.Contains(m.data, "预览版") || !strings.Contains(m.data, "Ctrl+C") {
						t.Errorf("%s：README.txt 缺内容：%q", x, m.data)
					}
					if (x.OS == "windows") != strings.Contains(m.data, "SmartScreen") {
						t.Errorf("%s：SmartScreen 的提示只在 Windows 上有", x)
					}
				}
			}
		}
	})
}

// buildAnydoor 照 O-R3 构建 ./cmd/anydoor（CGO_ENABLED=0、-trimpath）。
func buildAnydoor(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), (Target{runtime.GOOS, runtime.GOARCH}).Exe("anydoor"))
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w -X main.version=9.9.9-test", "-o", bin, "./cmd/anydoor")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("构建 ./cmd/anydoor：%v\n%s", err, out)
	}
	return bin
}

func withSetting(bi *debug.BuildInfo, key, value string) *debug.BuildInfo {
	c := *bi
	c.Settings = nil
	found := false
	for _, s := range bi.Settings {
		if s.Key == key {
			s.Value, found = value, true
			if value == "" {
				continue
			}
		}
		c.Settings = append(c.Settings, s)
	}
	if !found && value != "" {
		c.Settings = append(c.Settings, debug.BuildSetting{Key: key, Value: value})
	}
	return &c
}

func TestInspect(t *testing.T) {
	t.Run("O03-T3 检查", func(t *testing.T) {
		bin := buildAnydoor(t)
		bi, err := buildinfo.ReadFile(bin)
		must(t, err)
		native := Target{runtime.GOOS, runtime.GOARCH}
		if err := checkBuildInfo(bi, native); err != nil {
			t.Errorf("按 O-R3 构建的程序应当通过：%v", err)
		}
		other := "amd64"
		if runtime.GOARCH == "amd64" {
			other = "arm64"
		}
		for name, c := range map[string]*debug.BuildInfo{
			"CGO_ENABLED=1": withSetting(bi, "CGO_ENABLED", "1"),
			"缺 -trimpath":   withSetting(bi, "-trimpath", ""),
			"GOARCH 不对":     withSetting(bi, "GOARCH", other),
		} {
			if err := checkBuildInfo(c, native); err == nil {
				t.Errorf("%s 应当失败", name)
			}
		}
		index := []byte("<!doctype html><script type=\"module\" src=\"/assets/a.js\"></script>")
		if err := checkEmbedded(append(append([]byte("前面"), index...), "后面"...), index); err != nil {
			t.Errorf("含有 index 全文时应当通过：%v", err)
		}
		if err := checkEmbedded([]byte("只有占位页"), index); err == nil {
			t.Error("不含 index 全文时应当失败")
		}
		if err := checkEmbedded(index, nil); err == nil {
			t.Error("index 是空的时应当失败")
		}
		if err := checkSums(bi, filepath.Join(repoRoot(t), "go.sum")); err != nil {
			t.Errorf("依赖的哈希应当和 go.sum 一致：%v", err)
		}
	})
}

// fakeSource 是照 B.02 §4–§5 行为的假 anydoor（只用标准库）；FAKE_SMOKE_MODE=noscript 时首页没有入口脚本，
// =noline 时不打印"已启动"那一行，=badversion 时 --version 是没注入的缺省值，=mismatch 时 health 报的版本和 --version 不同。
const fakeSource = `package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	version := flag.Bool("version", false, "")
	flag.Bool("no-browser", false, "")
	flag.Bool("fake-llm", false, "")
	flag.Int("port", 0, "")
	flag.String("data-dir", "", "")
	flag.Parse()
	if *version {
		if os.Getenv("FAKE_SMOKE_MODE") == "badversion" {
			fmt.Println("anydoor 0.0.0-dev")
			fmt.Println("提交：unknown")
			fmt.Println("构建时间：1970-01-01T00:00:00Z")
			return
		}
		fmt.Println("anydoor 0.1.0-preview-dev.3")
		fmt.Println("提交：0123456789abcdef0123456789abcdef01234567")
		fmt.Println("构建时间：2026-11-24T08:00:00Z")
		fmt.Println("支持的 schema 版本：图文档 1.0")
		return
	}
	mode := os.Getenv("FAKE_SMOKE_MODE")
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		os.Exit(2)
	}
	const tok = "faketoken"
	stop := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		v := "0.1.0-preview-dev.3"
		if mode == "mismatch" {
			v = "9.9.9"
		}
		fmt.Fprint(w, ` + "`" + `{"ok":true,"version":"` + "`" + `+v+` + "`" + `"}` + "`" + `)
	})
	mux.HandleFunc("POST /api/v1/admin/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+tok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		stop <- struct{}{}
	})
	mux.HandleFunc("GET /assets/app.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fmt.Fprint(w, "console.log(1)")
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") == tok {
			http.SetCookie(w, &http.Cookie{Name: "hx_session", Value: "s", Path: "/", HttpOnly: true})
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		if mode == "noscript" {
			fmt.Fprint(w, "<html>前端还没有构建</html>")
			return
		}
		fmt.Fprint(w, ` + "`" + `<!doctype html><script type="module" crossorigin src="/assets/app.js"></script>` + "`" + `)
	})
	go func() { _ = http.Serve(ln, mux) }()
	addr := fmt.Sprintf("http://%s/?t=%s", ln.Addr(), tok)
	if mode == "noline" {
		fmt.Println("启动了：" + addr)
	} else {
		fmt.Println("Anywhere Door 已启动：" + addr)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	select {
	case <-sig:
	case <-stop:
		time.Sleep(50 * time.Millisecond)
	}
}
`

func buildFake(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fakeanydoor\n\ngo 1.27\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(fakeSource), 0o644))
	bin := filepath.Join(dir, (Target{runtime.GOOS, runtime.GOARCH}).Exe("fake"))
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("编译假程序：%v\n%s", err, out)
	}
	return bin
}

func TestParseVersion(t *testing.T) {
	t.Run("O03-T4 冒烟：--version 的格式", func(t *testing.T) {
		good := "anydoor 0.1.0-preview\r\n提交：0123456789abcdef0123456789abcdef01234567\r\n构建时间：2026-11-24T08:00:00Z\r\n支持的 schema 版本：图文档 1.0\r\n"
		if v, err := parseVersion(good); err != nil || v != "0.1.0-preview" {
			t.Errorf("合规的输出应当通过并读出版本号：%q %v", v, err)
		}
		for name, s := range map[string]string{
			"缺省版本号":     strings.Replace(good, "0.1.0-preview", "0.0.0-dev", 1),
			"版本号为空":     strings.Replace(good, "anydoor 0.1.0-preview", "anydoor ", 1),
			"提交不是哈希":    strings.Replace(good, "0123456789abcdef0123456789abcdef01234567", "unknown", 1),
			"缺省时间":      strings.Replace(good, "2026-11-24T08:00:00Z", "1970-01-01T00:00:00Z", 1),
			"没有 schema": strings.Replace(good, "支持的 schema 版本：图文档 1.0", "", 1),
		} {
			if _, err := parseVersion(s); err == nil {
				t.Errorf("%s 应当失败", name)
			}
		}
	})
}

func TestSmoke(t *testing.T) {
	t.Run("O03-T4 冒烟", func(t *testing.T) {
		bin := buildFake(t)
		old := startTimeout
		startTimeout = 3 * time.Second
		defer func() { startTimeout = old }()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		for mode, want := range map[string]string{"ok": "", "noscript": "第 3 步", "noline": "第 2 步", "badversion": "第 1 步", "mismatch": "第 3 步"} {
			t.Setenv("FAKE_SMOKE_MODE", mode)
			err := Smoke(ctx, bin)
			switch {
			case want == "" && err != nil:
				t.Errorf("%s：照 B.02 行为的程序应当通过：%v", mode, err)
			case want != "" && (err == nil || !strings.HasPrefix(err.Error(), want)):
				t.Errorf("%s：应当在%s失败，得到 %v", mode, want, err)
			}
		}
	})
}

// gitEnv 让测试里的 git 不受本机全局配置影响。
func gitEnv(t *testing.T) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	must(t, os.WriteFile(cfg, nil, 0o644))
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL": cfg, "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com",
		"GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com",
	} {
		t.Setenv(k, v)
	}
}

func TestVersionCheck(t *testing.T) {
	t.Run("O03-T5 版本检查", func(t *testing.T) {
		gitEnv(t)
		dir := t.TempDir()
		git := func(args ...string) {
			t.Helper()
			cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v：%v\n%s", args, err, out)
			}
		}
		git("init", "-q")
		must(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644))
		git("add", "a.txt")
		git("commit", "-q", "-m", "init")
		if err := checkVersion(dir); err == nil {
			t.Error("不在标签上应当失败")
		}
		git("tag", "v0.1.0")
		if err := checkVersion(dir); err != nil {
			t.Errorf("正好在一个 v* 标签上、工作区干净应当通过：%v", err)
		}
		must(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644))
		if err := checkVersion(dir); err == nil {
			t.Error("工作区不干净应当失败")
		}
		must(t, os.Remove(filepath.Join(dir, "b.txt")))
		git("tag", "v0.1.1")
		if err := checkVersion(dir); err == nil {
			t.Error("同一个提交上有两个 v* 标签应当失败")
		}
	})
}
