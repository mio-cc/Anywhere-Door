// Package messages 嵌入给人看的文案（C.11 §6、G.13）：internal/api 启动时读 errors.zh.yaml（B.03.05，P.09 K242）。
// 本目录的 yaml 仍是唯一来源；tools/gen 照旧从这里生成前端的文案。
package messages

import "embed"

// FS 是本目录下的全部 *.zh.yaml。
//
//go:embed *.zh.yaml
var FS embed.FS
