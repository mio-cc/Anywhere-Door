// STUB(B.08): 控制台编码（非 Windows 什么也不做）。随 anydoorctl 的桩一起整体替换（G.02.06-R10）。

//go:build !windows

package main

func utf8Console() (restore func()) { return func() {} }
