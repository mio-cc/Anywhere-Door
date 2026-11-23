package config

import (
	"flag"
	"fmt"
	"io"
)

// Args 是 anydoor 的命令行参数（G.11 §2、B.02 §4）。
type Args struct {
	DataDir, LogLevel                string
	Port                             *int // 没写 --port 时为 nil
	NoBrowser, Dev, FakeLLM, Version bool
}

// ParseArgs 只认 G.11 §2 的参数和 --version；不认识的参数返回错误。
func ParseArgs(args []string) (Args, error) {
	fs := flag.NewFlagSet("anydoor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var a Args
	fs.StringVar(&a.DataDir, "data-dir", "", "数据目录")
	fs.StringVar(&a.LogLevel, "log-level", "", "日志级别")
	port := fs.Int("port", 0, "端口")
	fs.BoolVar(&a.NoBrowser, "no-browser", false, "不打开浏览器")
	fs.BoolVar(&a.Dev, "dev", false, "开发模式")
	fs.BoolVar(&a.FakeLLM, "fake-llm", false, "演示模式")
	fs.BoolVar(&a.Version, "version", false, "打印版本")
	if err := fs.Parse(args); err != nil {
		return Args{}, fmt.Errorf("config: 命令行参数：%w", err)
	}
	if fs.NArg() > 0 {
		return Args{}, fmt.Errorf("config: 不认识的参数 %q", fs.Arg(0))
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			p := *port
			a.Port = &p
		}
	})
	return a, nil
}
