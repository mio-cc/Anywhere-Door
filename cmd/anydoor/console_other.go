//go:build !windows

package main

func utf8Console() (restore func()) { return func() {} }
