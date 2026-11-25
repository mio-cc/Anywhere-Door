package graph

import (
	"bytes"
	"encoding/json"
)

// 文档的顺序：FromDoc 不改输入数组的顺序，Doc 原样导出；Scope 的查询结果按 ID 排序（E.02.01 §4）。
// 规范化序列化和哈希在 hash.go（E.02.03）。

// encode 序列化文档（不做 HTML 转义，开放对象里的原文保持不变）。
func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
