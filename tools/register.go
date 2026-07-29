package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Register adds all el-mcp-server tools to the given MCP server.
func Register(s *mcp.Server) {
	registerSpecTools(s)
	registerNetworkTools(s)
	registerProductTools(s)
	registerUITools(s)
}

// Capabilities returns the server capabilities required by el-mcp-server,
// including the MCP Apps (SEP-1865) UI extension advertisement, on top of
// the SDK's historical default of a bare logging capability.
func Capabilities() *mcp.ServerCapabilities {
	caps := &mcp.ServerCapabilities{
		Logging: &mcp.LoggingCapabilities{},
	}
	caps.AddExtension("io.modelcontextprotocol/ui", map[string]any{
		"mimeTypes": []string{MCPAppsUIMimeType},
	})
	return caps
}
