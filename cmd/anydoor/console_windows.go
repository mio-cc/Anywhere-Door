//go:build windows

package main

import "syscall"

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleOutputCP = kernel32.NewProc("GetConsoleOutputCP")
	procSetConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
)

const cpUTF8 = 65001

// utf8Console 把控制台的输出代码页设为 UTF-8，返回恢复原代码页的函数。
// 中文 Windows 的控制台缺省是 936（GBK），不设的话双击运行时中文全是乱码（P.09 K198）。
// 没有控制台（输出被重定向）时什么也不做。
func utf8Console() (restore func()) {
	old, _, _ := procGetConsoleOutputCP.Call()
	if old == 0 || old == cpUTF8 {
		return func() {}
	}
	if r, _, _ := procSetConsoleOutputCP.Call(cpUTF8); r == 0 {
		return func() {}
	}
	return func() { _, _, _ = procSetConsoleOutputCP.Call(old) }
}
