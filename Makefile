.DEFAULT_GOAL := help

.PHONY: help build test mcpb clean

# release version embedded in .mcpb file names (JST date, same as the release tag)
VERSION ?= $(shell TZ=Asia/Tokyo date +%Y%m%d)

# .mcpb targets as GOOS/GOARCH/mcpb-platform
MCPB_TARGETS := darwin/arm64/darwin windows/amd64/win32

## show this help
help:
	@make2help $(MAKEFILE_LIST)

## build the el-mcp-server binary
build:
	go build -o el-mcp-server .

## run tests
test:
	go test ./...

## cross-build the .mcpb bundles (darwin_arm64, windows_amd64)
mcpb:
	@set -e; for t in $(MCPB_TARGETS); do \
		goos=$${t%%/*}; rest=$${t#*/}; goarch=$${rest%%/*}; platform=$${rest#*/}; \
		dir=mcpb/build/$${goos}_$${goarch}; \
		bin=el-mcp-server; if [ "$$goos" = windows ]; then bin=el-mcp-server.exe; fi; \
		out=mcpb/el-mcp-server_$(VERSION)_$${goos}_$${goarch}.mcpb; \
		echo "==> $$out"; \
		rm -rf $$dir; mkdir -p $$dir; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go build -o $$dir/$$bin .; \
		go run ./cmd/gen-mcpb-manifest -platform $$platform -o $$dir/manifest.json; \
		npx --yes @anthropic-ai/mcpb pack $$dir $$out; \
	done

## remove build artifacts
clean:
	rm -rf el-mcp-server mcpb/build mcpb/*.mcpb
