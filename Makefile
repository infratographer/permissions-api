BIN?=permissions-api

# Utility settings
ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
TOOLS_DIR := .tools

# Container build settings
CONTAINER_BUILD_CMD?=docker build

# Container settings
CONTAINER_REPO?=ghcr.io/infratographer
PERMISSIONS_API_CONTAINER_IMAGE_NAME = $(CONTAINER_REPO)/permissions-api
CONTAINER_TAG?=latest

# NATS settings
NATS_CREDS?=/tmp/user.creds

# Tool Versions
GCI_REPO = github.com/daixiang0/gci
GCI_VERSION = v0.10.1

GOLANGCI_LINT_REPO = github.com/golangci/golangci-lint
GOLANGCI_LINT_VERSION = v1.59.1

NATS_CLI_REPO = github.com/nats-io/natscli
NATS_CLI_VERSION = v0.0.35

NATS_NSC_VERSION = v2.8.0

NATS_NK_REPO = github.com/nats-io/nkeys
NATS_NK_VERSION = latest

ZED_REPO = github.com/authzed/zed
ZED_VERSION = v0.10.1

# go files to be checked
GO_FILES=$(shell git ls-files '*.go')

## Targets

.PHONY: help
help: Makefile ## Print help
	@grep -h "##" $(MAKEFILE_LIST) | grep -v grep | sed -e 's/:.*##/#/' | column -c 2 -t -s#

.PHONY: build
build:  ## Builds permissions-api binary.
	go build -o $(BIN) ./main.go

.PHONY: ci
ci: | golint test coverage  ## Setup dev database and run tests.

.PHONY: test
test:  ## Runs unit tests.
	@echo Running unit tests...
	@go test -v -timeout 120s -cover -short -tags testtools ./...

.PHONY: coverage
coverage:  ## Generates a test coverage report.
	@echo Generating coverage report...
	@go test -timeout 120s -tags testtools ./... -coverprofile=coverage.out -covermode=atomic
	@go tool cover -func=coverage.out
	@go tool cover -html=coverage.out

lint: golint  ## Runs all lint checks.

golint: | vendor $(TOOLS_DIR)/golangci-lint  ## Runs Go lint checks.
	@echo Linting Go files...
	@$(TOOLS_DIR)/golangci-lint run --timeout 5m

clean:  ## Cleans generated files.
	@echo Cleaning...
	@rm -f coverage.out
	@go clean -testcache
	@rm -rf $(TOOLS_DIR)

vendor:  ## Downloads and tidies go modules.
	@go mod download
	@go mod tidy

.PHONY: gci-diff gci-write gci
gci-diff: $(GO_FILES) | $(TOOLS_DIR)/gci  ## Outputs improper go import ordering.
	@$(TOOLS_DIR)/gci diff -s 'standard,default,prefix(github.com/infratographer)' $^

gci-write: $(GO_FILES) | $(TOOLS_DIR)/gci  ## Checks and updates all go files for proper import ordering.
	@$(TOOLS_DIR)/gci write -s 'standard,default,prefix(github.com/infratographer)' $^

gci: | gci-diff gci-write  ## Outputs and corrects all improper go import ordering.

.PHONY: nats-account
nats-account: | $(TOOLS_DIR)/nsc ## Generates NATS user account credentials.
	@sudo chown -Rh vscode:vscode $(ROOT_DIR)/.devcontainer/nsc
	@echo "Dumping NATS user creds file"
	@$(TOOLS_DIR)/nsc --data-dir=$(ROOT_DIR)/.devcontainer/nsc/nats/nsc/stores generate creds -a INFRADEV -n USER > $(NATS_CREDS)

image:  ## Builds all docker images.
	$(CONTAINER_BUILD_CMD) -f Dockerfile . -t $(PERMISSIONS_API_CONTAINER_IMAGE_NAME):$(CONTAINER_TAG)

.PHONY: dev-infra-up
dev-infra-up:  ## Starts local services to simplify local development.
	@echo Starting services
	@pushd .devcontainer && docker compose up -d --wait && popd

.PHONY: dev-infra-down
dev-infra-down:  ## Stops local services used for local development.
	@echo Stopping services
	@pushd .devcontainer && docker compose down && popd

$(TOOLS_DIR):
	mkdir -p $(TOOLS_DIR)

$(TOOLS_DIR)/gci: | $(TOOLS_DIR)
	@echo "Installing $(GCI_REPO)@$(GCI_VERSION)"
	@GOBIN=$(ROOT_DIR)/$(TOOLS_DIR) go install $(GCI_REPO)@$(GCI_VERSION)
	$@ --version

$(TOOLS_DIR)/golangci-lint: | $(TOOLS_DIR)
	@echo "Installing $(GOLANGCI_LINT_REPO)/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)"
	@GOBIN=$(ROOT_DIR)/$(TOOLS_DIR) go install $(GOLANGCI_LINT_REPO)/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$@ version
	$@ linters

$(TOOLS_DIR)/nsc: | $(TOOLS_DIR)
	@echo "Installing NATS tooling"
	@curl -o $(TOOLS_DIR)/nats_install.sh https://raw.githubusercontent.com/nats-io/nsc/$(NATS_NSC_VERSION)/install.sh
	@chmod +x $(TOOLS_DIR)/nats_install.sh
	@$(TOOLS_DIR)/nats_install.sh -s $(ROOT_DIR)/$(TOOLS_DIR)
	@rm $(TOOLS_DIR)/nats_install.sh

$(TOOLS_DIR)/nats: | $(TOOLS_DIR)
	@GOBIN=$(ROOT_DIR)/$(TOOLS_DIR) go install $(NATS_CLI_REPO)/nats@$(NATS_CLI_VERSION)

$(TOOLS_DIR)/zed: | $(TOOLS_DIR)
	@GOBIN=$(ROOT_DIR)/$(TOOLS_DIR) go install $(ZED_REPO)/cmd/zed@$(ZED_VERSION)

.PHONY: tools
tools: $(TOOLS_DIR)/gci $(TOOLS_DIR)/golangci-lint $(TOOLS_DIR)/nsc $(TOOLS_DIR)/nats $(TOOLS_DIR)/zed ## Installs development tools.
