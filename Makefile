# ============================================================
# Makefile — pixe-go
# module: github.com/cwlls/pixe-go
# ============================================================
# Build logic (ldflags, target platforms, archive formats) is defined
# exclusively in .goreleaser.yaml. The Makefile delegates all binary
# compilation to GoReleaser so there is a single source of build truth.
# ============================================================

# ---------- variables ---------------------------------------
BINARY      := pixe
BUILD_DIR   := .

# Test flags
TEST_FLAGS  := -race -timeout 120s
COVER_OUT   := coverage.out
COVER_HTML  := coverage.html

# Tools
GOLANGCI    := golangci-lint

# ---------- default target ----------------------------------
.DEFAULT_GOAL := help

# ---------- phony targets -----------------------------------
.PHONY: help build build-debug run clean test test-unit test-integration test-all \
        test-cover test-cover-html lint vet fmt fmt-check tidy deps check install uninstall

# ---------- help --------------------------------------------
help: ## Show this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	     /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ---------- build -------------------------------------------
build: ## Build pixe for the current platform via GoReleaser (snapshot)
	goreleaser build --single-target --snapshot --clean -o $(BUILD_DIR)/$(BINARY)

build-debug: ## Build without stripping symbols (for dlv) — bypasses GoReleaser
	go build -gcflags "all=-N -l" -o $(BUILD_DIR)/$(BINARY) .

# ---------- run ---------------------------------------------
run: build ## Build then run pixe with ARGS (e.g. make run ARGS="sort --help")
	./$(BINARY) $(ARGS)

# ---------- clean -------------------------------------------
clean: ## Remove build artifacts, GoReleaser dist/, and coverage files
	rm -f $(BUILD_DIR)/$(BINARY)
	rm -rf dist/
	rm -f $(COVER_OUT) $(COVER_HTML)

# ---------- test --------------------------------------------
test: test-unit ## Alias for test-unit

test-unit: ## Run unit tests (excludes integration)
	go test $(TEST_FLAGS) $(shell go list ./... | grep -v '/integration')

test-integration: ## Run integration tests only (requires build)
	go test $(TEST_FLAGS) -v ./internal/integration/...

test-all: ## Run all tests including integration
	go test $(TEST_FLAGS) ./...

test-cover: ## Run unit tests with coverage report
	go test $(TEST_FLAGS) -coverprofile=$(COVER_OUT) -covermode=atomic \
	    $(shell go list ./... | grep -v '/integration')
	go tool cover -func=$(COVER_OUT)

test-cover-html: test-cover ## Open HTML coverage report in browser
	go tool cover -html=$(COVER_OUT) -o $(COVER_HTML)
	open $(COVER_HTML)

# ---------- code quality ------------------------------------
vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go source files
	gofmt -w -s .

fmt-check: ## Check formatting without modifying files (CI-safe)
	@out=$$(gofmt -l -s .); \
	if [ -n "$$out" ]; then \
	    echo "The following files need formatting:"; \
	    echo "$$out"; \
	    exit 1; \
	fi

lint: ## Run golangci-lint (install: brew install golangci-lint)
	$(GOLANGCI) run ./...

check: fmt-check vet test-unit ## Run fmt-check + vet + unit tests (fast CI gate)

# ---------- dependencies ------------------------------------
tidy: ## Run go mod tidy
	go mod tidy

deps: ## Download all module dependencies
	go mod download

# ---------- install / uninstall -----------------------------
install: build ## Install pixe to $GOPATH/bin (or $GOBIN)
	cp $(BUILD_DIR)/$(BINARY) $(shell go env GOPATH)/bin/$(BINARY)

uninstall: ## Remove pixe from $GOPATH/bin
	rm -f $(shell go env GOPATH)/bin/$(BINARY)
