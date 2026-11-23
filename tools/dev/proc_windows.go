//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"syscall"
)

var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

// setProcAttr 让子进程自成一个进程组，CTRL_BREAK 能送到它和它启动的所有进程（go run 的子程序、pnpm 启动的 node）。
func setProcAttr(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func interrupt(c *exec.Cmd) error {
	if c.Process == nil {
		return nil
	}
	r, _, err := procGenerateConsoleCtrlEvent.Call(uintptr(syscall.CTRL_BREAK_EVENT), uintptr(c.Process.Pid))
	if r == 0 {
		return err
	}
	return nil
}

// kill 强制结束整棵进程树（没有控制台、CTRL_BREAK 送不到时也能结束）。
func kill(c *exec.Cmd) error {
	if c.Process == nil {
		return nil
	}
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(c.Process.Pid)).Run()
}
