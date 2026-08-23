.DEFAULT_GOAL := help

.PHONY: help build test mcpb clean

## show this help
help:
	@make2help $(MAKEFILE_LIST)

## build the el-mcp-server binary
build:
	go build -o el-mcp-server .

## run tests
test:
	go test ./...

## build the .mcpb bundle (macOS only)
mcpb:
	@if [ "$$(uname -s)" != "Darwin" ]; then \
		echo "error: mcpb target supports macOS (darwin) only" >&2; \
		exit 1; \
	fi
	go build -o mcpb/el-mcp-server .
	npx --yes @anthropic-ai/mcpb pack mcpb mcpb/el-mcp-server.mcpb
	@echo "==> done: mcpb/el-mcp-server.mcpb"

## remove build artifacts
clean:
	rm -rf el-mcp-server mcpb/el-mcp-server mcpb/el-mcp-server.mcpb
