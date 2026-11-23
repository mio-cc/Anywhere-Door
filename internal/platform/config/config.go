// Package config 加载、合并、校验、监视、写入配置（B.07.03、G.11）：命令行 > 环境变量 > 配置文件 > 默认值。
package config

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

// Source 是一个值的来源。
type Source string

// 值的来源。
const (
	FromDefault Source = "default"
	FromFile    Source = "file"
	FromEnv     Source = "env"
	FromFlag    Source = "flag"
)

// Options 是 Load 的选项。
type Options struct {
	Logger        *slog.Logger
	WatchInterval time.Duration // 缺省 2 秒
}

// Config 是合并后的配置；创建后不再修改（热更新得到新的 *Config）。
type Config struct {
	vals     map[string]any
	src      map[string]Source
	pkgs     map[string]any
	warnings []string
	pending  []string

	args    Args
	environ []string
	d       datadir.Dir
	o       Options
	stamp   stamp
	startup map[string]any    // Restart 项启动时的值
	startSr map[string]Source // 以及来源
}

type stamp struct {
	exists bool
	mod    time.Time
	size   int64
}

const fileName = "anydoor.yaml"

// Load 合并四层配置。数据目录 d 已经由 datadir.Resolve 定好。
func Load(a Args, environ []string, d datadir.Dir, o Options) (*Config, error) {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.WatchInterval <= 0 {
		o.WatchInterval = 2 * time.Second
	}
	b, st, err := readFile(d.Config())
	if err != nil {
		return nil, err
	}
	c, err := build(a, environ, d, b, st.exists)
	if err != nil {
		return nil, err
	}
	c.o, c.stamp = o, st
	c.startup, c.startSr = map[string]any{}, map[string]Source{}
	for _, it := range Items {
		if it.Hot == Restart {
			c.startup[it.Path], c.startSr[it.Path] = c.vals[it.Path], c.src[it.Path]
		}
	}
	for _, w := range c.warnings {
		o.Logger.Warn("config: " + w)
	}
	return c, nil
}

func readFile(p string) ([]byte, stamp, error) {
	fi, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, stamp{}, nil
	}
	if err != nil {
		return nil, stamp{}, fmt.Errorf("config: 读不了 %s：%w", fileName, err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, stamp{}, fmt.Errorf("config: 读不了 %s：%w", fileName, err)
	}
	return b, stamp{exists: true, mod: fi.ModTime(), size: fi.Size()}, nil
}

var posRe = regexp.MustCompile(`\[(\d+):(\d+)\]`)

// syntaxErr 把 goccy 的错误（"[行:列] 原因"）写成"第 N 行：原因"。
func syntaxErr(err error) error {
	msg := strings.TrimSpace(strings.SplitN(err.Error(), "\n", 2)[0])
	if m := posRe.FindStringSubmatch(msg); m != nil {
		reason := strings.TrimSpace(posRe.ReplaceAllString(msg, ""))
		return fmt.Errorf("config: %s 第 %s 行：%s", fileName, m[1], reason)
	}
	return fmt.Errorf("config: %s：%s", fileName, msg)
}

func build(a Args, environ []string, d datadir.Dir, b []byte, exists bool) (*Config, error) {
	c := &Config{vals: map[string]any{}, src: map[string]Source{}, pkgs: map[string]any{}, args: a, environ: environ, d: d}
	for _, it := range Items {
		c.vals[it.Path], c.src[it.Path] = cloneValue(it.Default), FromDefault
	}
	c.vals["data.dir"] = d.Root
	if a.DataDir != "" {
		c.src["data.dir"] = FromFlag
	}
	if exists {
		if err := c.applyFile(b, d.Root); err != nil {
			return nil, err
		}
	}
	if err := c.applyEnv(environ, d.Root); err != nil {
		return nil, err
	}
	if err := c.applyFlags(a); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) warn(format string, args ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}

func (c *Config) applyFile(b []byte, base string) error {
	f, err := parser.ParseBytes(b, parser.ParseComments)
	if err != nil {
		return syntaxErr(err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return syntaxErr(err)
	}
	return c.walk(f, "", raw, base)
}

func (c *Config) walk(f *ast.File, prefix string, m map[string]any, base string) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p, v := k, m[k]
		if prefix != "" {
			p = prefix + "." + k
		}
		if it, ok := byPath[p]; ok {
			switch {
			case it.Hot == ReadOnly:
				c.warn("%s 是只读项，忽略配置文件里的值", p)
			case p == "data.dir":
				c.warn("data.dir 不能写在配置文件里（用 --data-dir 或 %s），已忽略", datadir.EnvDataDir)
			default:
				nv, err := coerce(it, v, base, true)
				if err != nil {
					return fmt.Errorf("config: %s%s：%v", fileName, lineOf(f, p), err)
				}
				c.vals[p], c.src[p] = nv, FromFile
			}
			continue
		}
		if p == "packages" {
			if sub, ok := asMap(v); ok {
				c.pkgs = sub
				continue
			}
		}
		if sub, ok := asMap(v); ok && isPrefix(p) {
			if err := c.walk(f, p, sub, base); err != nil {
				return err
			}
			continue
		}
		c.warn("不认识的配置项 %s，原样保留", p)
	}
	return nil
}

// lineOf 是 " 第 N 行 <路径>"；找不到行号时只有路径。
func lineOf(f *ast.File, p string) string {
	if n := find(f, strings.Split(p, ".")); n != nil {
		if tk := n.GetToken(); tk != nil && tk.Position != nil {
			return fmt.Sprintf(" 第 %d 行 %s", tk.Position.Line, p)
		}
	}
	return " " + p
}

func asMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, x := range m {
			out[fmt.Sprint(k)] = x
		}
		return out, true
	}
	return nil, false
}

func (c *Config) applyEnv(environ []string, base string) error {
	env := map[string]string{}
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}
	for _, it := range Items {
		if !it.Env {
			continue
		}
		name := EnvName(it.Path)
		s, ok := env[name]
		if !ok {
			continue
		}
		if it.Hot == ReadOnly {
			c.warn("%s 是只读项，忽略环境变量 %s", it.Path, name)
			continue
		}
		v, err := parseEnv(it, s)
		if err == nil {
			v, err = coerce(it, v, base, true)
		}
		if err != nil {
			return fmt.Errorf("config: 环境变量 %s：%v", name, err)
		}
		c.vals[it.Path], c.src[it.Path] = v, FromEnv
	}
	return nil
}

func parseEnv(it Item, s string) (any, error) {
	switch it.Kind {
	case Int:
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return nil, errors.New("应当是整数")
		}
		return n, nil
	case Bool:
		b, err := strconv.ParseBool(strings.TrimSpace(s))
		if err != nil {
			return nil, errors.New("应当是 true 或 false")
		}
		return b, nil
	case String:
		return s, nil
	}
	var v any
	if err := yaml.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("不是合法的 YAML：%v", err)
	}
	return v, nil
}

func (c *Config) applyFlags(a Args) error {
	if a.Port != nil {
		v, err := coerce(byPath["server.port"], *a.Port, "", false)
		if err != nil {
			return fmt.Errorf("config: --port：%v", err)
		}
		c.vals["server.port"], c.src["server.port"] = v, FromFlag
	}
	if a.NoBrowser {
		c.vals["server.open_browser"], c.src["server.open_browser"] = false, FromFlag
	}
	if a.LogLevel != "" {
		v, err := coerce(byPath["log.level"], a.LogLevel, "", false)
		if err != nil {
			return fmt.Errorf("config: --log-level：%v", err)
		}
		c.vals["log.level"], c.src["log.level"] = v, FromFlag
	}
	if a.FakeLLM {
		c.vals["models.demo_mode"], c.src["models.demo_mode"] = true, FromFlag
	}
	return nil
}

// coerce 把 v 规整成 it 的类型并检查枚举和范围；expand 时展开路径项。
func coerce(it Item, v any, base string, expand bool) (any, error) {
	switch it.Kind {
	case Int:
		n, ok := toInt(v)
		if !ok {
			return nil, errors.New("应当是整数")
		}
		if it.Min != nil && n < *it.Min {
			return nil, fmt.Errorf("不能小于 %d", *it.Min)
		}
		if it.Max != nil && n > *it.Max {
			return nil, fmt.Errorf("不能大于 %d", *it.Max)
		}
		return n, nil
	case Bool:
		b, ok := v.(bool)
		if !ok {
			return nil, errors.New("应当是 true 或 false")
		}
		return b, nil
	case String:
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("应当是字符串")
		}
		if len(it.Enum) > 0 && !slices.Contains(it.Enum, s) {
			return nil, fmt.Errorf("只能是 %s 之一", strings.Join(it.Enum, "、"))
		}
		if it.IsPath && expand && s != "" {
			return datadir.ExpandPath(s, base)
		}
		return s, nil
	case Strings:
		var list []string
		switch x := v.(type) {
		case []string:
			list = append([]string{}, x...)
		case []any:
			for _, e := range x {
				s, ok := e.(string)
				if !ok {
					return nil, errors.New("应当是字符串列表")
				}
				list = append(list, s)
			}
		case nil:
		default:
			return nil, errors.New("应当是字符串列表")
		}
		if list == nil {
			list = []string{}
		}
		if it.IsPath && expand {
			for i, s := range list {
				p, err := datadir.ExpandPath(s, base)
				if err != nil {
					return nil, err
				}
				list[i] = p
			}
		}
		return list, nil
	}
	return v, nil
}

func toInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case int32:
		return int64(n), true
	case uint64:
		if n > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case uint32:
		return int64(n), true
	case float64:
		if n != math.Trunc(n) || math.Abs(n) > 1<<53 {
			return 0, false
		}
		return int64(n), true
	}
	return 0, false
}

func cloneValue(v any) any {
	switch x := v.(type) {
	case []string:
		return append([]string{}, x...)
	case []any:
		return append([]any{}, x...)
	}
	return v
}

// Get 取一个值；packages.<id>.<key> 返回文件里的值，没有时 ok 为 false。
func (c *Config) Get(path string) (any, bool) {
	if v, ok := c.vals[path]; ok {
		return cloneValue(v), true
	}
	rest, ok := strings.CutPrefix(path, "packages.")
	if !ok {
		return nil, false
	}
	var cur any = c.pkgs
	for _, seg := range strings.Split(rest, ".") {
		m, ok := asMap(cur)
		if !ok {
			return nil, false
		}
		if cur, ok = m[seg]; !ok {
			return nil, false
		}
	}
	return cur, true
}

// Int 取整数项。
func (c *Config) Int(path string) int64 { n, _ := c.vals[path].(int64); return n }

// Bool 取布尔项。
func (c *Config) Bool(path string) bool { b, _ := c.vals[path].(bool); return b }

// String 取字符串项。
func (c *Config) String(path string) string { s, _ := c.vals[path].(string); return s }

// Strings 取字符串列表项。
func (c *Config) Strings(path string) []string {
	l, _ := c.vals[path].([]string)
	return append([]string{}, l...)
}

// Source 是值的来源。
func (c *Config) Source(path string) Source { return c.src[path] }

// Warnings 是加载时的警告。
func (c *Config) Warnings() []string { return append([]string{}, c.warnings...) }

// Pending 是文件里改了、要重启才生效的路径。
func (c *Config) Pending() []string { return append([]string{}, c.pending...) }

// Watch 阻塞到 ctx 结束；配置文件变了就重新读文件层，有变化时回调 onChange。
func (c *Config) Watch(ctx context.Context, onChange func(*Config)) {
	t := time.NewTicker(c.o.WatchInterval)
	defer t.Stop()
	cur, last := c, c.stamp
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		b, st, err := readFile(c.d.Config())
		if err != nil {
			c.o.Logger.Warn(err.Error())
			continue
		}
		if st == last {
			continue
		}
		last = st
		next, err := build(cur.args, cur.environ, cur.d, b, st.exists)
		if err != nil {
			c.o.Logger.Warn("config: 配置文件改坏了，保持原来的配置", "err", err)
			continue
		}
		next.o, next.stamp, next.startup, next.startSr = cur.o, st, cur.startup, cur.startSr
		for _, it := range Items {
			if it.Hot != Restart {
				continue
			}
			if !reflect.DeepEqual(next.vals[it.Path], cur.startup[it.Path]) {
				next.pending = append(next.pending, it.Path)
				next.vals[it.Path], next.src[it.Path] = cur.startup[it.Path], cur.startSr[it.Path]
			}
		}
		if reflect.DeepEqual(next.vals, cur.vals) && reflect.DeepEqual(next.pkgs, cur.pkgs) && slices.Equal(next.pending, cur.pending) {
			continue
		}
		for _, w := range next.warnings {
			c.o.Logger.Warn("config: " + w)
		}
		cur = next
		onChange(next)
	}
}
