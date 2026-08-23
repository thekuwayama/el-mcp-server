// gen-mcpb-manifest regenerates the "tools" array of mcpb/manifest.json from
// the tools actually registered by tools.Register, so the .mcpb bundle's
// advertised tool list can't drift from the server implementation.
//
// Run from the repository root:
//
//	go run ./cmd/gen-mcpb-manifest
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/thekuwayama/el-mcp-server/tools"
)

const manifestPath = "mcpb/manifest.json"

// manifest mirrors the field order of mcpb/manifest.json so re-marshaling
// keeps the file's structure stable; only Tools is regenerated.
type manifest struct {
	ManifestVersion string         `json:"manifest_version"`
	Name            string         `json:"name"`
	DisplayName     string         `json:"display_name"`
	Version         string         `json:"version"`
	Description     string         `json:"description"`
	LongDescription string         `json:"long_description"`
	Author          author         `json:"author"`
	License         string         `json:"license"`
	Repository      repository     `json:"repository"`
	Homepage        string         `json:"homepage"`
	Keywords        []string       `json:"keywords"`
	Compatibility   compatibility  `json:"compatibility"`
	Server          serverConfig   `json:"server"`
	Tools           []manifestTool `json:"tools"`
}

type author struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type repository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type compatibility struct {
	Platforms []string `json:"platforms"`
}

type serverConfig struct {
	Type       string    `json:"type"`
	EntryPoint string    `json:"entry_point"`
	MCPConfig  mcpConfig `json:"mcp_config"`
}

type mcpConfig struct {
	Command string `json:"command"`
}

type manifestTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func main() {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing %s: %v\n", manifestPath, err)
		os.Exit(1)
	}

	registeredTools, err := listRegisteredTools(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	m.Tools = registeredTools

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	out = append(out, '\n')

	if err := os.WriteFile(manifestPath, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%d tools written to %s\n", len(m.Tools), manifestPath)
}

// listRegisteredTools spins up the real server and an in-memory client to
// call the standard tools/list RPC, so the extracted tool set matches
// exactly what an MCP client would see over the wire.
func listRegisteredTools(ctx context.Context) ([]manifestTool, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "el-mcp-server",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		Capabilities: tools.Capabilities(),
	})
	tools.Register(server)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		return nil, fmt.Errorf("connecting server transport: %w", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "gen-mcpb-manifest", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting client: %w", err)
	}
	defer session.Close()

	var result []manifestTool
	cursor := ""
	for {
		res, err := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, fmt.Errorf("listing tools: %w", err)
		}
		for _, t := range res.Tools {
			result = append(result, manifestTool{Name: t.Name, Description: t.Description})
		}
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}

	return result, nil
}
