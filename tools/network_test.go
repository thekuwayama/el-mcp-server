package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestFormElicitationSupported(t *testing.T) {
	cases := []struct {
		name string
		caps *mcp.ElicitationCapabilities
		want bool
	}{
		{"no elicitation capability", nil, false},
		{"empty capability (backward compat)", &mcp.ElicitationCapabilities{}, true},
		{"form supported", &mcp.ElicitationCapabilities{Form: &mcp.FormElicitationCapabilities{}}, true},
		{"url only", &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}}, false},
		{"form and url", &mcp.ElicitationCapabilities{Form: &mcp.FormElicitationCapabilities{}, URL: &mcp.URLElicitationCapabilities{}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formElicitationSupported(c.caps); got != c.want {
				t.Errorf("formElicitationSupported(%+v) = %v, want %v", c.caps, got, c.want)
			}
		})
	}
}

// callConfirmTool connects an in-memory client/server pair on the given
// protocol version and calls a tool whose handler runs confirmPropertyWrite
// with the request's client capabilities and params, so the multi round-trip
// confirmation is exercised end to end through the SDK without touching UDP.
// It returns the tool's text output ("confirmed" or "cancelled").
func callConfirmTool(t *testing.T, protocolVersion string, epc string, elicitationHandler func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error)) (string, error) {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "0.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "confirm"}, func(_ context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		confirmed, pending := confirmPropertyWrite(req.ClientCapabilities(), req.Params, "192.168.1.100", "027D01", epc, "30")
		if pending != nil {
			return pending, nil, nil
		}
		if confirmed {
			return textResult("confirmed"), nil, nil
		}
		return textResult("cancelled"), nil, nil
	})

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, &mcp.ClientOptions{
		ElicitationHandler: elicitationHandler,
	})
	cs, err := client.Connect(ctx, clientTransport, &mcp.ClientSessionOptions{ProtocolVersion: protocolVersion})
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "confirm"})
	if err != nil {
		return "", err
	}
	if len(res.Content) != 1 {
		t.Fatalf("CallTool() content = %v, want one text content", res.Content)
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool() content[0] = %T, want *mcp.TextContent", res.Content[0])
	}
	return text.Text, nil
}

func TestConfirmPropertyWrite(t *testing.T) {
	accept := func(confirm map[string]any) func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		return func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "accept", Content: confirm}, nil
		}
	}
	cases := []struct {
		name    string
		handler func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error)
		want    string
	}{
		{"no elicitation handler skips confirmation", nil, "confirmed"},
		{"user accepts with confirm=true", accept(map[string]any{"confirm": true}), "confirmed"},
		{"user accepts default (confirm omitted, defaults false)", accept(map[string]any{}), "cancelled"},
		{"user declines", func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "decline"}, nil
		}, "cancelled"},
	}
	// "" negotiates the latest protocol (input requests are returned to the
	// client); 2025-11-25 exercises the SDK's elicitation/create fallback.
	for _, pv := range []string{"", "2025-11-25"} {
		for _, c := range cases {
			t.Run(c.name+" (protocol "+pv+")", func(t *testing.T) {
				got, err := callConfirmTool(t, pv, "80", c.handler)
				if err != nil {
					t.Fatalf("CallTool() error = %v", err)
				}
				if got != c.want {
					t.Errorf("CallTool() = %q, want %q", got, c.want)
				}
			})
		}

		t.Run("message shows the formatted EPC (protocol "+pv+")", func(t *testing.T) {
			var msg string
			_, epc, errRes := parseEOJAndEPC("013001", " b0 ")
			if errRes != nil {
				t.Fatal("parseEOJAndEPC() returned an error result")
			}
			if _, err := callConfirmTool(t, pv, formatEPC(epc), func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
				msg = req.Params.Message
				return &mcp.ElicitResult{Action: "decline"}, nil
			}); err != nil {
				t.Fatalf("CallTool() error = %v", err)
			}
			if !strings.Contains(msg, "EPC B0 に") {
				t.Errorf("elicitation message = %q, want it to contain %q", msg, "EPC B0 に")
			}
		})

		t.Run("client elicitation error fails the call (protocol "+pv+")", func(t *testing.T) {
			wantErr := errors.New("boom")
			_, err := callConfirmTool(t, pv, "80", func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
				return nil, wantErr
			})
			if err == nil {
				t.Fatal("CallTool() error = nil, want non-nil")
			}
		})
	}

	t.Run("answer for a different write asks again", func(t *testing.T) {
		caps := &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{}}
		_, first := confirmPropertyWrite(caps, &mcp.CallToolParamsRaw{}, "192.168.1.100", "027D01", "80", "30")
		if first == nil {
			t.Fatal("confirmPropertyWrite() pending = nil, want an input request")
		}
		params := &mcp.CallToolParamsRaw{
			InputResponses: mcp.InputResponseMap{confirmInputKey: &mcp.ElicitResult{Action: "accept", Content: map[string]any{"confirm": true}}},
			RequestState:   first.RequestState,
		}
		if confirmed, pending := confirmPropertyWrite(caps, params, "192.168.1.100", "027D01", "80", "30"); !confirmed || pending != nil {
			t.Errorf("same write: confirmed = %v, pending = %v, want true, nil", confirmed, pending)
		}
		if confirmed, pending := confirmPropertyWrite(caps, params, "192.168.1.100", "027D01", "80", "31"); confirmed || pending == nil {
			t.Errorf("different EDT: confirmed = %v, pending = %v, want false, non-nil", confirmed, pending)
		}
	})
}
