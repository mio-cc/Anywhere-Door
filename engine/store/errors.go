package store

import "errors"

// 哨兵错误，调用方用 errors.Is 判断（E.07.01 §7）。
var (
	ErrNotFound  = errors.New("store: not found")
	ErrConflict  = errors.New("store: conflict") // 唯一约束、版本不连续、分支已存在
	ErrLeaseHeld = errors.New("store: lease held")
)
