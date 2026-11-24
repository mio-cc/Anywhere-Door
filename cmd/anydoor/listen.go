package main

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"
)

// listen 只监听回环地址（S.02-R1、INV-21）：先 127.0.0.1，再尽量监听 [::1] 的同一端口（没有 IPv6 不算失败）。
// port 为 0 时先试 7300，被占用则由系统分配（B.02 §5"监听"）。
func listen(port int, log *slog.Logger) ([]net.Listener, error) {
	var ln net.Listener
	var err error
	if port != 0 {
		ln, err = net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	} else if ln, err = net.Listen("tcp4", "127.0.0.1:7300"); err != nil {
		ln, err = net.Listen("tcp4", "127.0.0.1:0")
	}
	if err != nil {
		if port == 0 {
			port = 7300
		}
		return nil, fmt.Errorf("端口 %d 被占用或无法监听：%w", port, err)
	}
	p := ln.Addr().(*net.TCPAddr).Port
	lns := []net.Listener{ln}
	if ln6, err := net.Listen("tcp6", net.JoinHostPort("::1", strconv.Itoa(p))); err == nil {
		lns = append(lns, ln6)
	} else {
		log.Debug("没有监听 [::1]", "err", err)
	}
	return lns, nil
}
