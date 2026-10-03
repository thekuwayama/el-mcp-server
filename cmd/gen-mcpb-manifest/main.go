// gen-mcpb-manifest generates a per-platform .mcpb manifest from
// mcpb/manifest.template.json. The template holds only the platform-independent
// metadata; compatibility, server and tools are filled in here, with tools taken
// from what tools.Register actually registers so the bundle's advertised tool
// list can't drift from the server implementation.
//
// Run from the repository root:
//
//	go run ./cmd/gen-mcpb-manifest -platform win32 -o mcpb/build/windows_amd64/manifest.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/thekuwayama/el-mcp-server/tools"
)

const templatePath = "mcpb/manifest.template.json"

// binaryNames maps a .mcpb platform (Node's process.platform value, as used by
// compatibility.platforms) to the server binary name bundled for it.
var binaryNames = map[string]string{
	"darwin": "el-mcp-server",
	"win32":  "el-mcp-server.exe",
}

// manifest mirrors the field order of the .mcpb manifest. Fields up to Keywords
// come from the template; Compatibility, Server and Tools are generated.
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
	platform := flag.String("platform", "", ".mcpb platform to generate the manifest for (darwin or win32)")
	outPath := flag.String("o", "", "output path for the generated manifest")
	flag.Parse()

	binaryName, ok := binaryNames[*platform]
	if !ok {
		fmt.Fprintf(os.Stderr, "error: -platform must be darwin or win32 (got %q)\n", *platform)
		os.Exit(1)
	}
	if *outPath == "" {
		fmt.Fprintln(os.Stderr, "error: -o is required")
		os.Exit(1)
	}

	raw, err := os.ReadFile(templatePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing %s: %v\n", templatePath, err)
		os.Exit(1)
	}

	registeredTools, err := listRegisteredTools(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	m.Compatibility = compatibility{Platforms: []string{*platform}}
	m.Server = serverConfig{
		Type:       "binary",
		EntryPoint: binaryName,
		MCPConfig:  mcpConfig{Command: "${__dirname}/" + binaryName},
	}
	m.Tools = registeredTools

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	out = append(out, '\n')

	if err := os.WriteFile(*outPath, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%d tools written to %s\n", len(m.Tools), *outPath)
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
