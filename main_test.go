package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thekuwayama/el-mcp-server/tools"
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

func TestHTTPHandlerRejectsCrossOriginRequests(t *testing.T) {
	srv := httptest.NewServer(newHTTPHandler(tools.NewServer("test")))
	defer srv.Close()

	const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`

	tests := []struct {
		name      string
		header    map[string]string
		forbidden bool
	}{
		{name: "non-browser client", header: nil, forbidden: false},
		{name: "same origin", header: map[string]string{"Sec-Fetch-Site": "same-origin"}, forbidden: false},
		{name: "cross site via Sec-Fetch-Site", header: map[string]string{"Sec-Fetch-Site": "cross-site"}, forbidden: true},
		{name: "cross origin via Origin", header: map[string]string{"Origin": "https://evil.example"}, forbidden: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(initialize))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			for k, v := range tt.header {
				req.Header.Set(k, v)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if got := resp.StatusCode == http.StatusForbidden; got != tt.forbidden {
				t.Errorf("status = %d, want forbidden = %v", resp.StatusCode, tt.forbidden)
			}
		})
	}
}
