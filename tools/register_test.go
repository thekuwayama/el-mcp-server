package tools

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestRegister guards against tool definitions that only fail at startup,
// e.g. a jsonschema struct tag the SDK rejects, which makes mcp.AddTool panic.
func TestRegister(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "0.0.0"}, &mcp.ServerOptions{Capabilities: Capabilities()})
	Register(s)
}
