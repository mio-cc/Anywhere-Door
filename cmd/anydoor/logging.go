package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

// openLog：M0 的日志用 log/slog 文本格式，同时写标准错误和 <DataDir>/logs/anydoor-YYYYMMDD.log
// （按启动当天的本地日期，追加写；B.02 §5，P.09 K242）。脱敏规则等 G.12 复查时补。
func openLog(d datadir.Dir, level string, stderr io.Writer, now time.Time) (*slog.Logger, func(), error) {
	p := filepath.Join(d.Logs(), "anydoor-"+now.Format("20060102")+".log")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("打不开日志文件 %s：%w", p, err)
	}
	var lv slog.Level
	if err := lv.UnmarshalText([]byte(level)); err != nil {
		lv = slog.LevelInfo
	}
	h := slog.NewTextHandler(io.MultiWriter(stderr, f), &slog.HandlerOptions{Level: lv})
	return slog.New(h), func() { _ = f.Close() }, nil
}
