.PHONY: tests

tests:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out -json ./... | tee test_output.json | \
	jq -r 'select(.Output != null) | .Output' | sed '/^\s*$$/d' | sed 's/^[ \t]*//'
	go tool cover -func=coverage.out

	@TEST_COUNT=$$(jq -r 'select(.Action == "pass" or .Action == "fail") | .Test' test_output.json | sort -u | wc -l); \
	COVERAGE=$$(go tool cover -func=coverage.out | grep total: | awk '{print $$3}'); \
	echo "Total executed tests: $$TEST_COUNT"; \
	echo "Code coverage: $$COVERAGE"

	@rm -f test_output.json coverage.out
