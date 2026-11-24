package main

import "embed"

// webDist 是构建时由 tools/build 复制进来的前端生产构建（O.01 §4.2）；没构建前端时只有占位文件 .gitkeep。
//
//go:embed all:dist
var webDist embed.FS
