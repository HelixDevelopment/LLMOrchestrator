.PHONY: test build vet clean help lint fmt race fuzz

## help: Show all available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'

## test: Run all tests with race detector
test:
	go test ./... -race -count=1

## build: Build all packages
build:
	go build ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run static analysis
lint:
	go vet ./...
	@echo "Lint complete"

## fmt: Format all Go files
fmt:
	gofmt -w -s .

## race: Run tests with race detector (verbose)
race:
	go test ./... -race -count=1 -v

## fuzz: Run fuzz tests for 30 seconds
fuzz:
	go test ./pkg/parser/ -fuzz=FuzzParser_Parse -fuzztime=30s
	go test ./pkg/parser/ -fuzz=FuzzParser_ExtractJSON -fuzztime=30s
	go test ./pkg/parser/ -fuzz=FuzzParser_ExtractActions -fuzztime=30s

## clean: Clean build cache
clean:
	go clean -cache -testcache

## cover: Run tests with coverage report
cover:
	go test ./... -race -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## bench: Run benchmarks
bench:
	go test ./... -bench=. -benchmem

## check: Run all quality checks (vet + test)
check: vet test

## upstream-push: Push to all 4 remotes
upstream-push:
	./upstreams/push-all.sh

## upstream-sync: Sync from all remotes
upstream-sync:
	./upstreams/sync-all.sh

# Definition of Done gates — portable drop-in from the parent project
.PHONY: no-silent-skips no-silent-skips-warn demo-all demo-all-warn demo-one ci-validate-all

no-silent-skips:
	@bash scripts/no-silent-skips.sh

no-silent-skips-warn:
	@NO_SILENT_SKIPS_WARN_ONLY=1 bash scripts/no-silent-skips.sh

demo-all:
	@bash scripts/demo-all.sh

demo-all-warn:
	@DEMO_ALL_WARN_ONLY=1 DEMO_ALLOW_TODO=1 bash scripts/demo-all.sh

demo-one:
	@DEMO_MODULES="$(MOD)" bash scripts/demo-all.sh

ci-validate-all: no-silent-skips-warn demo-all-warn
	@echo "ci-validate-all: all gates executed"
