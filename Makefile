BIN?=permissions-api

# Utility settings
TOOLS_DIR := .tools
GOLANGCI_LINT_VERSION = v1.50.1

# Container build settings
CONTAINER_BUILD_CMD?=docker build

# Container settings
CONTAINER_REPO?=ghcr.io/infratographer
PERMISSIONS_API_CONTAINER_IMAGE_NAME = $(CONTAINER_REPO)/permissions-api
CONTAINER_TAG?=latest

# go files to be checked
GO_FILES=$(shell git ls-files '*.go')

## Targets

.PHONY: help
help: Makefile ## Print help
	@grep -h "##" $(MAKEFILE_LIST) | grep -v grep | sed -e 's/:.*##/#/' | column -c 2 -t -s#

.PHONY: build
build:  ## Builds permissions-api binary.
	go build -o $(BIN) ./main.go

.PHONY: test
test:  ## Runs unit tests.
	@echo Running unit tests...
	@go test -timeout 30s -cover -short  -tags testtools ./...

.PHONY: coverage
coverage:  ## Generates a test coverage report.
	@echo Generating coverage report...
	@go test -timeout 30s -tags testtools ./... -coverprofile=coverage.out -covermode=atomic
	@go tool cover -func=coverage.out
	@go tool cover -html=coverage.out

lint: golint  ## Runs all lint checks.

golint: | vendor $(TOOLS_DIR)/golangci-lint  ## Runs Go lint checks.
	@echo Linting Go files...
	@$(TOOLS_DIR)/golangci-lint run

clean:  ## Cleans generated files.
	@echo Cleaning...
	@rm -f coverage.out
	@go clean -testcache
	@rm -rf $(TOOLS_DIR)

vendor:  ## Downloads and tidies go modules.
	@go mod download
	@go mod tidy

.PHONY: gci-diff gci-write gci
gci-diff: $(GO_FILES) | gci-tool  ## Outputs improper go import ordering.
	@gci diff -s 'standard,default,prefix(github.com/infratographer)' $^

gci-write: $(GO_FILES) | gci-tool  ## Checks and updates all go files for proper import ordering.
	@gci write -s 'standard,default,prefix(github.com/infratographer)' $^

gci: | gci-diff gci-write  ## Outputs and corrects all improper go import ordering.

image:  ## Builds all docker images.
	$(CONTAINER_BUILD_CMD) -f Dockerfile . -t $(PERMISSIONS_API_CONTAINER_IMAGE_NAME):$(CONTAINER_TAG)

# Tools setup
$(TOOLS_DIR):
	mkdir -p $(TOOLS_DIR)

$(TOOLS_DIR)/golangci-lint: $(TOOLS_DIR)
	export \
		VERSION=$(GOLANGCI_LINT_VERSION) \
		URL=https://raw.githubusercontent.com/golangci/golangci-lint \
		BINDIR=$(TOOLS_DIR) && \
	curl -sfL $$URL/$$VERSION/install.sh | sh -s $$VERSION
	$(TOOLS_DIR)/golangci-lint version
	$(TOOLS_DIR)/golangci-lint linters

.PHONY: gci-tool
gci-tool:
	@which gci &>/dev/null \
		|| echo Installing gci tool \
		&& go install github.com/daixiang0/gci@latest

.PHONY: nk-tool
nk-tool:
	@which nk &>/dev/null || \
		echo Installing "nk" tool && \
		go install github.com/nats-io/nkeys/nk@latest && \
		export PATH=$$PATH:$(shell go env GOPATH)
