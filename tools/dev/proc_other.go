//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setProcAttr 让子进程自成一个进程组，信号能送到它和它启动的所有进程。
func setProcAttr(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func interrupt(c *exec.Cmd) error {
	if c.Process == nil {
		return nil
	}
	return syscall.Kill(-c.Process.Pid, syscall.SIGINT)
}

func kill(c *exec.Cmd) error {
	if c.Process == nil {
		return nil
	}
	return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
}
