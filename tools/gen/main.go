// Command gen 从 schema/ 生成 Go 和 TypeScript 代码（设计文档 C.14、G.16）。
//
//	go run ./tools/gen          生成全部生成物（在仓库根目录运行）
//	go run ./tools/gen tokens   令牌、主题（源文件还没有，C.14 §2）
//	go run ./tools/gen icons    图标表（同上）
//	go run ./tools/gen fonts    字体子集（同上）
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "tokens", "icons", "fonts":
			fmt.Fprintf(os.Stderr, "gen %s：还没有源文件（C.14 §2、G.16），什么也没有生成\n", os.Args[1])
			os.Exit(1)
		default:
			fmt.Fprintln(os.Stderr, "用法：go run ./tools/gen [tokens|icons|fonts]")
			os.Exit(2)
		}
	}
	outs, err := Generate(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen："+err.Error())
		os.Exit(1)
	}
	changed := 0
	for _, o := range outs {
		p := filepath.FromSlash(o.Path)
		if old, err := os.ReadFile(p); err == nil && bytes.Equal(old, o.Data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "gen："+err.Error())
			os.Exit(1)
		}
		if err := os.WriteFile(p, o.Data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "gen："+err.Error())
			os.Exit(1)
		}
		changed++
	}
	fmt.Printf("gen：%d 个生成物，改了 %d 个\n", len(outs), changed)
}
