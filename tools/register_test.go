package tools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestRegister guards against tool definitions that only fail at startup,
// e.g. a jsonschema struct tag the SDK rejects, which makes mcp.AddTool panic.
func TestRegister(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "0.0.0"}, &mcp.ServerOptions{Capabilities: Capabilities()})
	Register(s)
}

// newRegisteredClientSession connects an in-memory client to a server with
// all tools and resources registered, as an MCP client would see them.
func newRegisteredClientSession(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "0.0.0"}, &mcp.ServerOptions{Capabilities: Capabilities()})
	Register(server)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ss, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestToolAnnotations checks that every tool that writes to a device is not
// marked read-only, since clients may run read-only tools without
// confirmation.
func TestToolAnnotations(t *testing.T) {
	cs := newRegisteredClientSession(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	// Tools that write to a device, with their expected DestructiveHint.
	writers := map[string]bool{
		"set_property":         true,
		"render_smartmeter_ui": false, // writes EPC E5 but restores it
	}

	seen := map[string]bool{}
	for _, tool := range res.Tools {
		seen[tool.Name] = true
		if tool.Annotations == nil {
			t.Errorf("%s: Annotations = nil", tool.Name)
			continue
		}
		wantDestructive, writes := writers[tool.Name]
		if tool.Annotations.ReadOnlyHint == writes {
			t.Errorf("%s: ReadOnlyHint = %v, want %v", tool.Name, tool.Annotations.ReadOnlyHint, !writes)
		}
		if !writes {
			continue
		}
		if tool.Annotations.DestructiveHint == nil {
			t.Errorf("%s: DestructiveHint = nil, want %v", tool.Name, wantDestructive)
		} else if *tool.Annotations.DestructiveHint != wantDestructive {
			t.Errorf("%s: DestructiveHint = %v, want %v", tool.Name, *tool.Annotations.DestructiveHint, wantDestructive)
		}
	}
	for name := range writers {
		if !seen[name] {
			t.Errorf("tool %s is not registered", name)
		}
	}
}
