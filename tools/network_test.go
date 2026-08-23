package tools

import (
	"context"
	"errors"
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

// newTestSession connects an in-memory client/server pair and returns the
// server-side session, so confirmPropertyWrite can be exercised against a
// real (non-mocked) *mcp.ServerSession without touching UDP.
func newTestSession(t *testing.T, elicitationHandler func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error)) *mcp.ServerSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "0.0.0"}, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSessionCh := make(chan *mcp.ServerSession, 1)
	go func() {
		ss, err := server.Connect(ctx, serverTransport, nil)
		if err == nil {
			serverSessionCh <- ss
		}
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, &mcp.ClientOptions{
		ElicitationHandler: elicitationHandler,
	})
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	ss := <-serverSessionCh
	t.Cleanup(func() { _ = ss.Close() })
	return ss
}

func TestConfirmPropertyWrite(t *testing.T) {
	t.Run("no elicitation handler skips confirmation", func(t *testing.T) {
		session := newTestSession(t, nil)
		confirmed, err := confirmPropertyWrite(context.Background(), session, "192.168.1.100", "027D01", "80", "30")
		if err != nil {
			t.Fatalf("confirmPropertyWrite() error = %v", err)
		}
		if !confirmed {
			t.Errorf("confirmPropertyWrite() = false, want true (should skip confirmation)")
		}
	})

	t.Run("user accepts with confirm=true", func(t *testing.T) {
		session := newTestSession(t, func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "accept", Content: map[string]any{"confirm": true}}, nil
		})
		confirmed, err := confirmPropertyWrite(context.Background(), session, "192.168.1.100", "027D01", "80", "30")
		if err != nil {
			t.Fatalf("confirmPropertyWrite() error = %v", err)
		}
		if !confirmed {
			t.Errorf("confirmPropertyWrite() = false, want true")
		}
	})

	t.Run("user accepts default (confirm omitted, defaults false)", func(t *testing.T) {
		session := newTestSession(t, func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "accept", Content: map[string]any{}}, nil
		})
		confirmed, err := confirmPropertyWrite(context.Background(), session, "192.168.1.100", "027D01", "80", "30")
		if err != nil {
			t.Fatalf("confirmPropertyWrite() error = %v", err)
		}
		if confirmed {
			t.Errorf("confirmPropertyWrite() = true, want false (schema default is false)")
		}
	})

	t.Run("user declines", func(t *testing.T) {
		session := newTestSession(t, func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "decline"}, nil
		})
		confirmed, err := confirmPropertyWrite(context.Background(), session, "192.168.1.100", "027D01", "80", "30")
		if err != nil {
			t.Fatalf("confirmPropertyWrite() error = %v", err)
		}
		if confirmed {
			t.Errorf("confirmPropertyWrite() = true, want false")
		}
	})

	t.Run("client elicitation error propagates", func(t *testing.T) {
		wantErr := errors.New("boom")
		session := newTestSession(t, func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return nil, wantErr
		})
		_, err := confirmPropertyWrite(context.Background(), session, "192.168.1.100", "027D01", "80", "30")
		if err == nil {
			t.Fatal("confirmPropertyWrite() error = nil, want non-nil")
		}
	})
}
