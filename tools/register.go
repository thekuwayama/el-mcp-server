package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Register adds all el-mcp-server tools to the given MCP server.
func Register(s *mcp.Server) {
	registerSpecTools(s)
	registerNetworkTools(s)
	registerProductTools(s)
}
