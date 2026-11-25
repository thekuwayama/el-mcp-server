.DEFAULT_GOAL := help

.PHONY: help build test mcpb server-json generate manufacturers mra clean

VERSION ?= $(shell TZ=Asia/Tokyo date +%Y%m%d)
LDFLAGS := -X main.version=$(VERSION)
MCPB_TARGETS := darwin/arm64/darwin windows/amd64/win32
RELEASE_URL := https://github.com/thekuwayama/el-mcp-server/releases/download
MANUFACTURERS_DIR := echonet/spec/manufacturers
MRA_DIR := echonet/spec/mra

## show this help
help:
	@make2help $(MAKEFILE_LIST)

## build the el-mcp-server binary
build:
	go build -ldflags "$(LDFLAGS)" -o el-mcp-server .

## run tests
test:
	go test ./...

## cross-build the .mcpb bundles (darwin_arm64, windows_amd64)
mcpb: node_modules
	@set -e; for t in $(MCPB_TARGETS); do \
		goos=$${t%%/*}; rest=$${t#*/}; goarch=$${rest%%/*}; platform=$${rest#*/}; \
		dir=mcpb/build/$${goos}_$${goarch}; \
		bin=el-mcp-server; if [ "$$goos" = windows ]; then bin=el-mcp-server.exe; fi; \
		out=mcpb/el-mcp-server_$(VERSION)_$${goos}_$${goarch}.mcpb; \
		echo "==> $$out"; \
		rm -rf $$dir; mkdir -p $$dir; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go build -ldflags "$(LDFLAGS)" -o $$dir/$$bin .; \
		go run ./cmd/gen-mcpb-manifest -platform $$platform -version $(VERSION) -o $$dir/manifest.json; \
		npx --no-install mcpb pack $$dir $$out; \
	done

node_modules: package.json package-lock.json
	npm ci --ignore-scripts --no-audit --no-fund
	@touch $@

## generate server.json for the MCP Registry
server-json:
	@set -e; pkgs='[]'; for t in $(MCPB_TARGETS); do \
		goos=$${t%%/*}; rest=$${t#*/}; goarch=$${rest%%/*}; \
		file=el-mcp-server_$(VERSION)_$${goos}_$${goarch}.mcpb; \
		sha=$$(shasum -a 256 mcpb/$$file | cut -d ' ' -f 1); \
		pkgs=$$(echo "$$pkgs" | jq -c --arg url "$(RELEASE_URL)/$(VERSION)/$$file" --arg sha "$$sha" \
			'. + [{registryType: "mcpb", identifier: $$url, fileSha256: $$sha, transport: {type: "stdio"}}]'); \
	done; \
	jq --arg v "$(VERSION)" --argjson pkgs "$$pkgs" '.version = $$v | .packages = $$pkgs' mcpb/server.template.json > server.json

## regenerate spec data (manufacturer codes, MRA)
generate: manufacturers mra

## regenerate manufacturer codes from echonet.jp
manufacturers:
	@set -e; url=$$(sed -n '2p' $(MANUFACTURERS_DIR)/VERSION); \
	tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	cp $(MANUFACTURERS_DIR)/codes.json $$tmp/codes.json.orig; \
	curl -fsSL -o $$tmp/list_code.xlsx "$$url"; \
	go run ./cmd/gen-manufacturers $$tmp/list_code.xlsx; \
	if ! cmp -s $$tmp/codes.json.orig $(MANUFACTURERS_DIR)/codes.json; then \
		printf 'list_code.xlsx (%s)\n%s\n' "$$(TZ=Asia/Tokyo date +%Y-%m-%d)" "$$url" > $(MANUFACTURERS_DIR)/VERSION; \
	fi

## re-fetch the MRA spec of the version recorded in echonet/spec/mra/VERSION
mra:
	go run ./cmd/update-mra "$$(sed -n '2p' $(MRA_DIR)/VERSION)" "$$(head -1 $(MRA_DIR)/VERSION)"

## remove build artifacts
clean:
	rm -rf el-mcp-server mcpb/build mcpb/*.mcpb server.json
