// STUB(B.02): 嵌入前端产物。只为让 O.01 能构建与冒烟；B.02 实施时整体替换（G.02.06-R10）。

package main

import "embed"

// webDist 是构建时由 tools/build 复制进来的前端生产构建（O.01 §4.2）；没构建前端时只有占位文件 .gitkeep。
//
//go:embed all:dist
var webDist embed.FS
