.PHONY: setup lint test build coverage clean

GO := go
GOLINT := golangci-lint

setup:
	$(GO) mod download
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install gotest.tools/gotestsum@latest

lint:
	$(GOLINT) run ./...

test:
	gotestsum --format pkgname-and-test-fails -- ./... -coverprofile=coverage.out -coverpkg=./...

coverage: test
	$(GO) run tools/coverage/main.go coverage.out 90

build:
	$(GO) build -o bin/hyperrr ./cmd/hyperrr
	$(GO) build -o bin/mission-control ./cmd/tui

clean:
	rm -rf bin/
	rm -f coverage.out
