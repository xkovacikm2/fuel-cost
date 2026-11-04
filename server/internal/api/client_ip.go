package api

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func clientAddress(r *http.Request, trusted []netip.Prefix) string {
	host := r.RemoteAddr
	if value, _, err := net.SplitHostPort(host); err == nil {
		host = value
	}
	current, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	isTrusted := func(address netip.Addr) bool {
		for _, prefix := range trusted {
			if prefix.Contains(address.Unmap()) {
				return true
			}
		}
		return false
	}
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for index := len(forwarded) - 1; index >= 0 && isTrusted(current); index-- {
		address, err := netip.ParseAddr(strings.TrimSpace(forwarded[index]))
		if err != nil {
			break
		}
		current = address
	}
	return current.Unmap().String()
}
