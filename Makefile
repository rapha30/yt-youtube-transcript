BINARY  := yt-transcript
PKG     := ./cmd/yt-transcript
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test test-race test-network vet fmt fmtcheck lint vuln sec cover run docker clean ci help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-14s %s\n", $$1, $$2}'

build: ## Build the binary
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) $(PKG)

test: ## Run offline tests
	go test ./...

test-race: ## Run tests with the race detector
	go test -race ./...

test-network: ## Run live YouTube integration tests
	go test -tags network -run TestNetwork -v ./transcript/

vet: ## go vet
	go vet ./...

fmt: ## Format the code
	gofmt -w .

fmtcheck: ## Fail if code is not gofmt-clean
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

lint: ## Run golangci-lint (includes gosec)
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./...

vuln: ## Scan dependencies/stdlib for known vulnerabilities
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

sec: ## Run gosec security scanner
	go run github.com/securego/gosec/v2/cmd/gosec@latest ./...

cover: ## Show test coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

run: ## Run the CLI (use ARGS="...")
	go run $(PKG) $(ARGS)

docker: ## Build the Docker image
	docker build -t $(BINARY):dev .

clean: ## Remove build artifacts
	rm -f $(BINARY) $(BINARY).exe coverage.out
	rm -rf dist

ci: fmtcheck vet test-race vuln ## Run the full local CI gate
