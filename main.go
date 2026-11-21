package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/thekuwayama/el-mcp-server/tools"
)

// defaultHTTPAddr is the default listen address for the HTTP transport. It is
// bound to loopback only: the server has no authentication and set_property
// changes real device state, so exposing it to the LAN must be an explicit
// choice (e.g. -addr :8080).
const defaultHTTPAddr = "127.0.0.1:8080"

// version is the server version reported to MCP clients. Release builds set
// it with -ldflags "-X main.version=<VERSION>" (see the Makefile); plain
// go build / go install leave it as "dev".
var version = "dev"

func main() {
	transport := flag.String("transport", "stdio", "Transport type: stdio or http")
	addr := flag.String("addr", defaultHTTPAddr, "Listen address for HTTP transport (use :8080 to listen on all interfaces; there is no authentication)")
	flag.Parse()

	server := tools.NewServer(version)

	switch *transport {
	case "stdio":
		log.SetOutput(os.Stderr)
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatalf("server error: %v", err)
		}

	case "http":
		handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
			return server
		}, nil)
		log.Printf("el-mcp-server listening on %s (HTTP/Streamable)", *addr)
		if err := http.ListenAndServe(*addr, handler); err != nil {
			log.Fatalf("http server error: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown transport: %s (use stdio or http)\n", *transport)
		os.Exit(1)
	}
}
