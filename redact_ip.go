package main

import (
	"net"
	"strings"
)

func redactIP (addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return  addr
	}

	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil {
		return  addr
	}

	slice := strings.Split(ip.String(), ".")
	slice = append(slice[:len(slice) - 1], "x")
	return  strings.Join(slice, ".")
}
