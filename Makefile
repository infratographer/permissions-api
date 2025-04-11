BIN?=permissions-api

# Container build settings
CONTAINER_BUILD_CMD?=docker build

# Container settings
CONTAINER_REPO?=ghcr.io/infratographer
PERMISSIONS_API_CONTAINER_IMAGE_NAME = $(CONTAINER_REPO)/permissions-api
CONTAINER_TAG?=latest

# NATS settings
NATS_CREDS?=/tmp/user.creds

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

golint: | vendor  ## Runs Go lint checks.
	@echo Linting Go files...
	@go tool golangci-lint run --timeout 5m

fixlint:
	@echo Fixing go imports
	@find . -type f -iname '*.go' | xargs go tool goimports -w -local go.infratographer.com/permissions-api

clean:  ## Cleans generated files.
	@echo Cleaning...
	@rm -f coverage.out
	@go clean -testcache

vendor:  ## Downloads and tidies go modules.
	@go mod download
	@go mod tidy

.PHONY: nats-account
nats-account: ## Generates NATS user account credentials.
	@sudo chown -Rh vscode:vscode $(ROOT_DIR)/.devcontainer/nsc
	@echo "Dumping NATS user creds file"
	@go tool nsc --data-dir=$(ROOT_DIR)/.devcontainer/nsc/nats/nsc/stores generate creds -a INFRADEV -n USER > $(NATS_CREDS)

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
