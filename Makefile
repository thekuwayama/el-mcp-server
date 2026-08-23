.PHONY: build mcpb clean

build:
	go build -o el-mcp-server .

mcpb:
	@if [ "$$(uname -s)" != "Darwin" ]; then \
		echo "error: mcpb target supports macOS (darwin) only" >&2; \
		exit 1; \
	fi
	go build -o mcpb/el-mcp-server .
	npx --yes @anthropic-ai/mcpb pack mcpb mcpb/el-mcp-server.mcpb
	@echo "==> done: mcpb/el-mcp-server.mcpb"

clean:
	rm -rf el-mcp-server mcpb/el-mcp-server mcpb/el-mcp-server.mcpb
