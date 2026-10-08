################################################################################
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                          #
# Please see LICENSE file for details.                                         #
################################################################################

.PHONY: all test test-integration build clean fmt

GIT_COMMIT=$(shell git rev-parse --short=8 HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO_VERSION=$(shell go version | awk '{print $$3}')
# UTC so it always increases; := so build-all stamps one number on every target.
BUILD_NUMBER:=$(shell date -u +%Y%m%d%H%M%S)
APP_PKG=github.com/PivotLLM/MCPFusion/app
LDFLAGS=-ldflags "-X $(APP_PKG).gitCommit=$(GIT_COMMIT) -X $(APP_PKG).buildTime=$(BUILD_TIME) -X $(APP_PKG).goVersion=$(GO_VERSION) -X $(APP_PKG).buildNumber=$(BUILD_NUMBER) -s -w"

all: test
	$(MAKE) build

test:
	./test.sh

# Live MCP tests: need a running server, APIKEY and probe (see tests/README.md).
test-integration:
	bash tests/run_all_tests.sh

build:
	go build $(LDFLAGS) -o mcpfusion .
	cd cmd/auth && go build -o fusion-oauth .

clean:
	rm -f mcpfusion cmd/auth/fusion-oauth
	go clean -testcache

fmt:
	golangci-lint fmt
	cd cmd/auth && golangci-lint fmt
