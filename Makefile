################################################################################
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                          #
# Please see LICENSE file for details.                                         #
################################################################################

.PHONY: all test test-integration build clean fmt

all: test
	$(MAKE) build

test:
	./test.sh

# Live MCP tests: need a running server, APIKEY and probe (see tests/README.md).
test-integration:
	bash tests/run_all_tests.sh

build:
	go build -o mcpfusion .
	cd cmd/auth && go build -o fusion-oauth .

clean:
	rm -f mcpfusion cmd/auth/fusion-oauth
	go clean -testcache

fmt:
	golangci-lint fmt
	cd cmd/auth && golangci-lint fmt
