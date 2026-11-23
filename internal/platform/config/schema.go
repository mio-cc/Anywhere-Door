package config

import "strings"

// Kind 是配置项的类型。
type Kind int

// 配置项的类型。
const (
	String  Kind = iota
	Int          // int64
	Bool         // bool
	Strings      // []string
	Any          // 任意 YAML 值
)

// Hot 是配置项能不能热更新（G.11-R4）。
type Hot int

// 热更新方式。
const (
	Restart  Hot = iota // 重启后生效
	Live                // 立即生效
	NextRun             // 下次运行生效
	ReadOnly            // 只读，永远是默认值
)

// Item 是一个配置项的登记（G.11-R1：只在这里登记）。
type Item struct {
	Path     string
	Kind     Kind
	Default  any
	Flag     string // 命令行参数名，没有为空
	Env      bool   // 能不能用环境变量设
	Hot      Hot
	Enum     []string
	Min, Max *int64
	IsPath   bool
}

func i64(v int64) *int64 { return &v }

func str(path, def string, hot Hot, enum ...string) Item {
	return Item{Path: path, Kind: String, Default: def, Env: true, Hot: hot, Enum: enum}
}

func boolean(path string, def bool, hot Hot) Item {
	return Item{Path: path, Kind: Bool, Default: def, Env: true, Hot: hot}
}

func integer(path string, def int64, hot Hot, lo, hi *int64) Item {
	return Item{Path: path, Kind: Int, Default: def, Env: true, Hot: hot, Min: lo, Max: hi}
}

// Items 是 G.11 §3 的全部配置项（合并写的行已拆开）。
var Items = []Item{
	{Path: "server.port", Kind: Int, Default: int64(0), Flag: "port", Env: true, Hot: Restart, Min: i64(0), Max: i64(65535)},
	{Path: "server.open_browser", Kind: Bool, Default: true, Flag: "no-browser", Env: true, Hot: Restart},
	{Path: "data.dir", Kind: String, Default: "", Flag: "data-dir", Hot: Restart, IsPath: true},
	integer("data.retention.blob_days", 30, Live, i64(1), nil),
	{Path: "log.level", Kind: String, Default: "info", Flag: "log-level", Env: true, Hot: Live, Enum: []string{"debug", "info", "warn", "error"}},
	str("ui.theme.palette", "paper", Live, "paper", "graphite", "whale", "amber"),
	str("ui.theme.mode", "light", Live, "light", "dark", "system"),
	str("models.default", "", Live),
	{Path: "models.providers", Kind: Any, Default: []any{}, Env: true, Hot: Live},
	integer("pools.global_concurrency", 8, NextRun, i64(1), nil),
	{Path: "tools.allowed_hosts", Kind: Strings, Default: []string{}, Env: true, Hot: Live},
	{Path: "mcp.servers", Kind: Any, Default: []any{}, Env: true, Hot: Live},
	boolean("telemetry.enabled", false, Live),
	str("general.send_key", "enter", Live, "enter", "mod"),
	boolean("notify.approval", true, Live),
	boolean("notify.run", true, Live),
	boolean("notify.connection", true, Live),
	boolean("notify.budget", true, Live),
	boolean("notify.desktop", false, Live),
	str("models.orchestrator", "", Live),
	str("chat.entry_graph", "", Live),
	str("models.default_effort", "medium", Live, "off", "minimal", "low", "medium", "high", "xhigh", "max"),
	{Path: "models.demo_mode", Kind: Bool, Default: false, Flag: "fake-llm", Env: true, Hot: Live},
	integer("authz.default_level", 1, Live, i64(0), i64(4)),
	boolean("approval.merge_similar", true, Live),
	integer("approval.timeout_minutes", 0, NextRun, i64(0), nil),
	str("sandbox.backend", "wazero", NextRun, "wazero", "docker"),
	integer("sandbox.timeout_s", 10, NextRun, i64(1), i64(30)),
	integer("sandbox.memory_mb", 256, NextRun, i64(16), i64(512)),
	boolean("sandbox.network", false, NextRun),
	{Path: "tools.file_roots", Kind: Strings, Default: []string{}, Env: true, Hot: Live, IsPath: true},
	boolean("mcp.allow_local_http", false, Live),
	integer("budget.monthly_micros", 0, Live, i64(0), nil),
	integer("budget.run_default_micros", 0, NextRun, i64(0), nil),
	str("onboarding.step", "welcome", Live),
	boolean("security.taint_guard", true, ReadOnly),
	boolean("server.local_only", true, ReadOnly),
}

var byPath = func() map[string]Item {
	m := make(map[string]Item, len(Items))
	for _, it := range Items {
		m[it.Path] = it
	}
	return m
}()

// Lookup 取登记的配置项。
func Lookup(path string) (Item, bool) {
	it, ok := byPath[path]
	return it, ok
}

// EnvName 是配置项对应的环境变量名（G.11-R2）。
func EnvName(path string) string {
	return "ANYDOOR_" + strings.ToUpper(strings.ReplaceAll(path, ".", "_"))
}

// isPrefix 判断 p 是不是某个登记项的上级（例如 "notify"）。
func isPrefix(p string) bool {
	for _, it := range Items {
		if strings.HasPrefix(it.Path, p+".") {
			return true
		}
	}
	return false
}
