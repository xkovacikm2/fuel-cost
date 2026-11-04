package api

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientAddressOnlyTrustsConfiguredProxy(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.8")
	if got := clientAddress(request, nil); got != "127.0.0.1" {
		t.Fatalf("untrusted proxy spoofed client: %s", got)
	}
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}
	if got := clientAddress(request, trusted); got != "203.0.113.8" {
		t.Fatalf("trusted proxy client = %s", got)
	}
	request.Header.Set("X-Forwarded-For", "192.0.2.9, 203.0.113.8")
	if got := clientAddress(request, trusted); got != "203.0.113.8" {
		t.Fatalf("did not select nearest untrusted client: %s", got)
	}
}
