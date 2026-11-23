package config

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func openDir(t *testing.T) datadir.Dir {
	t.Helper()
	d, err := datadir.Open(t.TempDir())
	must(t, err)
	return d
}

// writeConfig 原子写配置文件（Watch 不会读到写了一半的文件）。
func writeConfig(t *testing.T, d datadir.Dir, s string) {
	t.Helper()
	must(t, datadir.WritePrivate(d.Config(), []byte(s)))
}

func load(t *testing.T, a Args, env []string, d datadir.Dir) *Config {
	t.Helper()
	c, err := Load(a, env, d, Options{})
	must(t, err)
	return c
}

func hasWarning(c *Config, s string) bool {
	for _, w := range c.Warnings() {
		if strings.Contains(w, s) {
			return true
		}
	}
	return false
}

func TestPriority(t *testing.T) {
	t.Run("B0703-T1 优先级", func(t *testing.T) {
		d := openDir(t)
		writeConfig(t, d, "server:\n  port: 7301\nlog:\n  level: warn\n")
		env := []string{"ANYDOOR_SERVER_PORT=7302", "ANYDOOR_LOG_LEVEL=error", "PATH=/bin"}
		a, err := ParseArgs([]string{"--port", "7303", "--log-level", "debug"})
		must(t, err)
		check := func(c *Config, port int64, level string, src Source) {
			t.Helper()
			if c.Int("server.port") != port || c.String("log.level") != level || c.Source("server.port") != src || c.Source("log.level") != src {
				t.Errorf("得到 %d %s（%s），应当 %d %s（%s）", c.Int("server.port"), c.String("log.level"), c.Source("server.port"), port, level, src)
			}
		}
		check(load(t, a, env, d), 7303, "debug", FromFlag)
		check(load(t, Args{}, env, d), 7302, "error", FromEnv)
		check(load(t, Args{}, nil, d), 7301, "warn", FromFile)
		check(load(t, Args{}, nil, openDir(t)), 0, "info", FromDefault)

		a, err = ParseArgs([]string{"--no-browser", "--fake-llm", "--dev"})
		must(t, err)
		c := load(t, a, nil, d)
		if c.Bool("server.open_browser") || !c.Bool("models.demo_mode") || c.Source("models.demo_mode") != FromFlag {
			t.Error("--no-browser、--fake-llm 的映射不对")
		}
		if !a.Dev || a.Port != nil {
			t.Errorf("ParseArgs 结果不对：%+v", a)
		}
		for _, bad := range [][]string{{"--bogus"}, {"extra"}, {"--port", "x"}} {
			if _, err := ParseArgs(bad); err == nil {
				t.Errorf("ParseArgs(%v) 应当报错", bad)
			}
		}
		if _, err := Load(Args{}, []string{"ANYDOOR_SERVER_PORT=70000"}, d, Options{}); err == nil {
			t.Error("环境变量超出范围应当报错")
		}
	})
}

func TestWatch(t *testing.T) {
	t.Run("B0703-T3 热更新", func(t *testing.T) {
		d := openDir(t)
		writeConfig(t, d, "server:\n  port: 7300\nlog:\n  level: info\n")
		c, err := Load(Args{}, nil, d, Options{WatchInterval: 20 * time.Millisecond})
		must(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		got := make(chan *Config, 8)
		go c.Watch(ctx, func(n *Config) {
			select {
			case got <- n:
			default:
			}
		})
		time.Sleep(50 * time.Millisecond)
		writeConfig(t, d, "server:\n  port: 7301\nlog:\n  level: debug\n")
		var n *Config
		select {
		case n = <-got:
		case <-time.After(time.Second):
			t.Fatal("1 秒内没有回调")
		}
		if n.String("log.level") != "debug" {
			t.Errorf("log.level 应当热更新为 debug，是 %s", n.String("log.level"))
		}
		if n.Int("server.port") != 7300 || !reflect.DeepEqual(n.Pending(), []string{"server.port"}) {
			t.Errorf("server.port 要重启才生效：%d，Pending %v", n.Int("server.port"), n.Pending())
		}
		if c.String("log.level") != "info" {
			t.Error("旧的 *Config 不应当变")
		}

		a, err := ParseArgs([]string{"--log-level", "error"})
		must(t, err)
		c2, err := Load(a, nil, d, Options{WatchInterval: 20 * time.Millisecond})
		must(t, err)
		got2 := make(chan *Config, 8)
		go c2.Watch(ctx, func(n *Config) {
			select {
			case got2 <- n:
			default:
			}
		})
		time.Sleep(50 * time.Millisecond)
		writeConfig(t, d, "server:\n  port: 7301\nlog:\n  level: warn\nui:\n  theme:\n    mode: dark\n")
		select {
		case n = <-got2:
		case <-time.After(time.Second):
			t.Fatal("1 秒内没有回调")
		}
		if n.String("log.level") != "error" || n.String("ui.theme.mode") != "dark" {
			t.Errorf("命令行设过的项不被文件覆盖：%s；其他项照常更新：%s", n.String("log.level"), n.String("ui.theme.mode"))
		}
	})
	t.Run("B0703-T4 坏文件", func(t *testing.T) {
		d := openDir(t)
		writeConfig(t, d, "server:\n  port: 7300\n bad: [\n")
		if _, err := Load(Args{}, nil, d, Options{}); err == nil || !strings.Contains(err.Error(), "第 ") || !strings.Contains(err.Error(), " 行") {
			t.Errorf("语法错误应当带行号：%v", err)
		}
		writeConfig(t, d, "server:\n  port: abc\n")
		if _, err := Load(Args{}, nil, d, Options{}); err == nil || !strings.Contains(err.Error(), "server.port") || !strings.Contains(err.Error(), "第 2 行") {
			t.Errorf("类型错误应当带路径和行号：%v", err)
		}

		writeConfig(t, d, "log:\n  level: info\n")
		c, err := Load(Args{}, nil, d, Options{WatchInterval: 20 * time.Millisecond})
		must(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		got := make(chan *Config, 8)
		go c.Watch(ctx, func(n *Config) {
			select {
			case got <- n:
			default:
			}
		})
		time.Sleep(50 * time.Millisecond)
		writeConfig(t, d, "log:\n  level: [broken\n")
		select {
		case n := <-got:
			t.Fatalf("改坏的文件不应当回调：%s", n.String("log.level"))
		case <-time.After(200 * time.Millisecond):
		}
		if c.String("log.level") != "info" {
			t.Error("旧配置应当照常可用")
		}
		writeConfig(t, d, "log:\n  level: debug\n")
		select {
		case n := <-got:
			if n.String("log.level") != "debug" {
				t.Errorf("改好后应当是 debug，是 %s", n.String("log.level"))
			}
		case <-time.After(time.Second):
			t.Fatal("改好后 1 秒内没有回调")
		}
	})
}

// g11Paths 是 G.11 §3 的全部路径（手写，与 schema.go 对照）。
var g11Paths = []string{
	"server.port", "server.open_browser", "data.dir", "data.retention.blob_days", "log.level",
	"ui.theme.palette", "ui.theme.mode", "models.default", "models.providers", "pools.global_concurrency",
	"tools.allowed_hosts", "mcp.servers", "telemetry.enabled", "general.send_key",
	"notify.approval", "notify.run", "notify.connection", "notify.budget", "notify.desktop",
	"models.orchestrator", "chat.entry_graph", "models.default_effort", "models.demo_mode",
	"authz.default_level", "approval.merge_similar", "approval.timeout_minutes",
	"sandbox.backend", "sandbox.timeout_s", "sandbox.memory_mb", "sandbox.network",
	"tools.file_roots", "mcp.allow_local_http", "budget.monthly_micros", "budget.run_default_micros",
	"onboarding.step", "security.taint_guard", "server.local_only",
}

// envValue 是一个合法的、能看出来源的环境变量值。
func envValue(it Item) string {
	switch it.Kind {
	case Int:
		return strconv.FormatInt(it.Default.(int64), 10)
	case Bool:
		return strconv.FormatBool(!it.Default.(bool))
	case String:
		if len(it.Enum) > 0 {
			return it.Enum[len(it.Enum)-1]
		}
		return "x"
	case Strings:
		return "[x]"
	}
	return "[]"
}

func TestRegistry(t *testing.T) {
	t.Run("B0703-T5 登记表", func(t *testing.T) {
		var got []string
		seen := map[string]bool{}
		for _, it := range Items {
			if seen[it.Path] {
				t.Errorf("%s 登记了两次", it.Path)
			}
			seen[it.Path] = true
			got = append(got, it.Path)
			if _, err := coerce(it, it.Default, "/base", false); err != nil {
				t.Errorf("%s 的默认值不合法：%v", it.Path, err)
			}
		}
		want := append([]string{}, g11Paths...)
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("登记表与 G.11 §3 不一致：\n得到 %v\n应当 %v", got, want)
		}
		if EnvName("server.port") != "ANYDOOR_SERVER_PORT" || EnvName("ui.theme.mode") != "ANYDOOR_UI_THEME_MODE" {
			t.Error("EnvName 不对")
		}

		d := openDir(t)
		var env []string
		for _, it := range Items {
			if it.Env && it.Hot != ReadOnly {
				env = append(env, EnvName(it.Path)+"="+envValue(it))
			}
		}
		c := load(t, Args{}, env, d)
		for _, it := range Items {
			if it.Env && it.Hot != ReadOnly && c.Source(it.Path) != FromEnv {
				t.Errorf("%s 用环境变量设置后来源是 %s", it.Path, c.Source(it.Path))
			}
		}

		writeConfig(t, d, "security:\n  taint_guard: false\ndata:\n  dir: /elsewhere\n")
		c = load(t, Args{}, []string{"ANYDOOR_SERVER_LOCAL_ONLY=false"}, d)
		if !c.Bool("security.taint_guard") || !c.Bool("server.local_only") {
			t.Error("只读项不能被改掉")
		}
		if !hasWarning(c, "security.taint_guard") || !hasWarning(c, "server.local_only") || !hasWarning(c, "data.dir") {
			t.Errorf("应当有只读项和 data.dir 的警告：%v", c.Warnings())
		}
		if c.String("data.dir") != d.Root {
			t.Errorf("data.dir 应当是 %s，是 %s", d.Root, c.String("data.dir"))
		}
	})
}

func TestValues(t *testing.T) {
	t.Run("B0703-T6 值的解析与路径", func(t *testing.T) {
		d := openDir(t)
		home, err := os.UserHomeDir()
		must(t, err)
		root := t.TempDir()
		t.Setenv("ANYDOOR_TEST_ROOT", root)
		writeConfig(t, d, "tools:\n  file_roots: [\"~/a\", \"$ANYDOOR_TEST_ROOT/b\", \"rel\"]\npackages:\n  demo_pkg:\n    greeting: hi\nmystery:\n  keep: 1\n")
		c := load(t, Args{}, []string{"ANYDOOR_TOOLS_ALLOWED_HOSTS=[a.com, b.com]", "ANYDOOR_TELEMETRY_ENABLED=true"}, d)
		if got := c.Strings("tools.allowed_hosts"); !reflect.DeepEqual(got, []string{"a.com", "b.com"}) {
			t.Errorf("环境变量里的 YAML 列表：%v", got)
		}
		if !c.Bool("telemetry.enabled") {
			t.Error("环境变量里的布尔值")
		}
		want := []string{filepath.Join(home, "a"), filepath.Join(root, "b"), filepath.Join(d.Root, "rel")}
		if got := c.Strings("tools.file_roots"); !reflect.DeepEqual(got, want) {
			t.Errorf("路径展开：%v，应当 %v", got, want)
		}
		if v, ok := c.Get("packages.demo_pkg.greeting"); !ok || v != "hi" {
			t.Errorf("packages 的值：%v %v", v, ok)
		}
		if _, ok := c.Get("packages.demo_pkg.missing"); ok {
			t.Error("没有的包配置应当 ok 为 false")
		}
		if !hasWarning(c, "mystery") {
			t.Errorf("未知项应当有警告：%v", c.Warnings())
		}
		writeConfig(t, d, "sandbox:\n  timeout_s: 60\n")
		if _, err := Load(Args{}, nil, d, Options{}); err == nil || !strings.Contains(err.Error(), "sandbox.timeout_s") {
			t.Errorf("超出范围应当报错：%v", err)
		}
	})
}
