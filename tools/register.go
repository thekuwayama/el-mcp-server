package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

// ServerName is the implementation name el-mcp-server reports to clients.
const ServerName = "el-mcp-server"

// NewServer creates the el-mcp-server MCP server with its capabilities and all
// tools registered. version is reported as the server implementation version;
// both the server binary and cmd/gen-mcpb-manifest build their server here so
// the two can't drift apart.
func NewServer(version string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    ServerName,
		Version: version,
	}, &mcp.ServerOptions{
		Capabilities: Capabilities(),
	})
	Register(s)
	return s
}

// Register adds all el-mcp-server tools to the given MCP server.
func Register(s *mcp.Server) {
	registerSpecTools(s)
	registerNetworkTools(s)
	registerProductTools(s)
	registerUITools(s)
	registerSkills(s)
}

// Capabilities returns the server capabilities required by el-mcp-server,
// including the MCP Apps (SEP-1865) UI and Skills over MCP (SEP-2640)
// extension advertisements, on top of
// the SDK's historical default of a bare logging capability.
func Capabilities() *mcp.ServerCapabilities {
	caps := &mcp.ServerCapabilities{
		Logging: &mcp.LoggingCapabilities{},
	}
	caps.AddExtension("io.modelcontextprotocol/ui", map[string]any{
		"mimeTypes": []string{MCPAppsUIMimeType},
	})
	caps.AddExtension(SkillsExtension, nil)
	return caps
}
