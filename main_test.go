package main

import (
	"net"
	"testing"
)

func TestDefaultHTTPAddrIsLoopback(t *testing.T) {
	host, _, err := net.SplitHostPort(defaultHTTPAddr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", defaultHTTPAddr, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		t.Errorf("defaultHTTPAddr = %q, want a loopback address", defaultHTTPAddr)
	}
}
