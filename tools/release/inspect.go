package main

import (
	"bufio"
	"bytes"
	"debug/buildinfo"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// Inspect 是每个平台都做的检查（O.03 §6.3，不运行程序）：构建信息、依赖版本与 go.sum 一致、嵌入了前端。
func Inspect(bin string, t Target) error {
	bi, err := buildinfo.ReadFile(bin)
	if err != nil {
		return fmt.Errorf("读不了 %s 的构建信息：%w", bin, err)
	}
	if err := checkBuildInfo(bi, t); err != nil {
		return err
	}
	if err := checkSums(bi, filepath.Join(root, "go.sum")); err != nil {
		return err
	}
	index, err := os.ReadFile(filepath.Join(root, "cmd", "anydoor", "dist", "index.html"))
	if err != nil {
		return fmt.Errorf("cmd/anydoor/dist 里没有 index.html：前端没有嵌入（先 pnpm -C web build）")
	}
	b, err := os.ReadFile(bin)
	if err != nil {
		return err
	}
	return checkEmbedded(b, index)
}

// checkBuildInfo：CGO_ENABLED=0、-trimpath、GOOS / GOARCH 与目标一致（O-R3）。
func checkBuildInfo(bi *debug.BuildInfo, t Target) error {
	s := map[string]string{}
	for _, kv := range bi.Settings {
		s[kv.Key] = kv.Value
	}
	var probs []string
	if s["CGO_ENABLED"] != "0" {
		probs = append(probs, fmt.Sprintf("CGO_ENABLED 是 %q，应当是 0", s["CGO_ENABLED"]))
	}
	if s["-trimpath"] != "true" {
		probs = append(probs, "没有用 -trimpath 构建")
	}
	if s["GOOS"] != t.OS || s["GOARCH"] != t.Arch {
		probs = append(probs, fmt.Sprintf("平台是 %s/%s，应当是 %s", s["GOOS"], s["GOARCH"], t))
	}
	if len(probs) > 0 {
		return errors.New("构建信息不对：" + strings.Join(probs, "；"))
	}
	return nil
}

// checkSums：程序里记录的每个依赖的版本和哈希都在 go.sum 里。
func checkSums(bi *debug.BuildInfo, sumFile string) error {
	f, err := os.Open(sumFile)
	if err != nil {
		return fmt.Errorf("读不了 go.sum：%w", err)
	}
	defer func() { _ = f.Close() }()
	known := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if fs := strings.Fields(sc.Text()); len(fs) == 3 {
			known[fs[0]+" "+fs[1]+" "+fs[2]] = true
		}
	}
	for _, d := range bi.Deps {
		if d.Replace != nil {
			d = d.Replace
		}
		if d.Sum != "" && !known[d.Path+" "+d.Version+" "+d.Sum] {
			return fmt.Errorf("依赖 %s %s 的哈希和 go.sum 不一致", d.Path, d.Version)
		}
	}
	return nil
}

// checkEmbedded：程序里能找到 cmd/anydoor/dist/index.html 的全文（go:embed 原样存进程序）。
func checkEmbedded(bin, index []byte) error {
	if len(index) == 0 {
		return errors.New("index.html 是空的")
	}
	if !bytes.Contains(bin, index) {
		return errors.New("程序里找不到嵌入的 index.html：前端没有嵌入")
	}
	return nil
}
