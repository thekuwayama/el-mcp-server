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

func main() {
	transport := flag.String("transport", "stdio", "Transport type: stdio or http")
	addr := flag.String("addr", ":8080", "Listen address for HTTP transport")
	flag.Parse()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "el-mcp-server",
		Version: "0.1.0",
	}, nil)

	tools.Register(server)

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
