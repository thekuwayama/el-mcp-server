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

// TestUIToolResources checks that every render_*_ui tool points at a
// registered ui:// dashboard resource served as an MCP Apps HTML document.
func TestUIToolResources(t *testing.T) {
	cs := newRegisteredClientSession(t)
	ctx := context.Background()

	rl, err := cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	resources := map[string]*mcp.Resource{}
	for _, r := range rl.Resources {
		resources[r.URI] = r
	}

	tl, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	got := map[string]string{}
	for _, tool := range tl.Tools {
		uiMeta, ok := tool.Meta["ui"].(map[string]any)
		if !ok {
			continue
		}
		uri, _ := uiMeta["resourceUri"].(string)
		got[tool.Name] = uri
	}

	want := map[string]string{
		"render_battery_ui":    batteryDashboardURI,
		"render_solar_ui":      solarDashboardURI,
		"render_v2h_ui":        v2hDashboardURI,
		"render_aircon_ui":     airconDashboardURI,
		"render_smartmeter_ui": smartmeterDashboardURI,
	}
	if len(got) != len(want) {
		t.Errorf("tools with _meta.ui = %v, want %v", got, want)
	}
	for name, uri := range want {
		if got[name] != uri {
			t.Errorf("%s: _meta.ui.resourceUri = %q, want %q", name, got[name], uri)
		}
		r, ok := resources[uri]
		if !ok {
			t.Errorf("%s: resource %s is not registered", name, uri)
			continue
		}
		if r.MIMEType != MCPAppsUIMimeType {
			t.Errorf("%s: MIMEType = %q, want %q", uri, r.MIMEType, MCPAppsUIMimeType)
		}
		rr, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
		if err != nil {
			t.Errorf("ReadResource(%s) error = %v", uri, err)
			continue
		}
		if len(rr.Contents) != 1 || rr.Contents[0].Text == "" || rr.Contents[0].MIMEType != MCPAppsUIMimeType {
			t.Errorf("ReadResource(%s) returned unexpected contents", uri)
		}
	}
}
