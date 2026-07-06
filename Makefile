ifeq ($(GO),)
	GO := $(shell which go 2>/dev/null || { [ -x /usr/local/go/bin/go ] && echo /usr/local/go/bin/go; } || echo go)
endif

ifndef GOPATH
	GOPATH := $(shell $(GO) env GOPATH 2>/dev/null || echo ~/go)
	export GOPATH
endif


SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

.PHONY: tests test-unit test-integration stats lint build clean

# Runs linters.
lint:
	$(GOPATH)/bin/golangci-lint run --config=.golangci.yml ./...

test-integration: test-integ-basic test-integ-ha

test-integ-basic:
	@chmod +x tests/integration_basic.sh
	./tests/integration_basic.sh

test-integ-ha:
	@chmod +x tests/integration_ha.sh
	./tests/integration_ha.sh

# Runs all tests.
tests: test-unit test-integ-basic test-integ-ha

# Runs unit tests.
test-unit:
	@echo "Running unit tests..."
	@$(GO) test -v -race -coverprofile=coverage.out -json ./... | tee test_output.json | \
	jq -r 'select(.Output != null) | .Output' | sed '/^\s*$$/d' | sed 's/^[ \t]*//'
	$(GO) tool cover -func=coverage.out

	@TEST_COUNT=$$(jq -r 'select(.Action == "pass" or .Action == "fail") | .Test' test_output.json | sort -u | wc -l); \
	COVERAGE=$$($(GO) tool cover -func=coverage.out | grep total: | awk '{print $$3}'); \
	echo "Total executed tests: $$TEST_COUNT"; \
	echo "Code coverage: $$COVERAGE"

	@rm -f test_output.json coverage.out

stats:
	@echo "Calculating Go code statistics (excluding tests)..."
	@TOTAL_LINES=$$(find . -name '*.go' ! -name '*_test.go' -print0 | xargs -0 cat | wc -l); \
	COMMENT_LINES=$$(find . -name '*.go' ! -name '*_test.go' -print0 | xargs -0 grep -E '^\s*//' | wc -l); \
	CODE_LINES=$$((TOTAL_LINES - COMMENT_LINES)); \
	echo "Total lines       : $$TOTAL_LINES"; \
	echo "Comment lines     : $$COMMENT_LINES"; \
	echo "Effective code lines: $$CODE_LINES"; \
	echo "Lint rules enabled: $$($(GOPATH)/bin/golangci-lint linters --json | jq '.Enabled | length')"

build:
	docker build -t coredns-gslb:latest .

clean:
	docker rmi -f coredns-gslb:latest || true

.PHONY: docs-serve
docs-serve:
	@echo "Starting documentation server locally via Docker..."
	@echo "Open http://localhost:8000 in your browser."
	docker run --rm -it -p 8000:8000 -v $$(pwd):/docs squidfunk/mkdocs-material serve --dev-addr 0.0.0.0:8000 --livereload