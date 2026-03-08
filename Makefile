## ─────────────────────────────────────────────────────────────────────────────
## KubePilot – Makefile
## ─────────────────────────────────────────────────────────────────────────────

BINARY        := kubepilot
IMAGE         := ghcr.io/kubepilot/kubepilot
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS       := -ldflags="-s -w -X github.com/kubepilot/kubepilot/internal/version.Version=$(VERSION)"

GO            := go
GOOS          ?= $(shell go env GOOS)
GOARCH        ?= $(shell go env GOARCH)

DASHBOARD_DIR := dashboard
DIST_DIR      := dist

.PHONY: all build build-linux build-darwin build-windows \
        dashboard dashboard-dev dashboard-install \
        generate manifests \
        docker-build docker-push \
        deploy-crds undeploy-crds \
        test lint vet \
        run-server run-operator \
        kind-up kind-down \
        clean help

## ── Default ────────────────────────────────────────────────────────────────
all: dashboard build

## ── Go binary ──────────────────────────────────────────────────────────────

# Build the kubepilot binary for the host platform.
build:
	@echo "▸ Building $(BINARY) ($(GOOS)/$(GOARCH))"
	@mkdir -p $(DIST_DIR)
	$(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY) ./cmd/kubepilot

# Cross-compile release binaries.
build-linux:
	@GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-amd64 ./cmd/kubepilot
	@GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-arm64 ./cmd/kubepilot

build-darwin:
	@GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-amd64 ./cmd/kubepilot
	@GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-arm64 ./cmd/kubepilot

build-windows:
	@GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-windows-amd64.exe ./cmd/kubepilot

## ── Code generation ────────────────────────────────────────────────────────

# Re-generate DeepCopy methods and CRD YAML manifests.
# Requires controller-gen: go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
generate:
	@echo "▸ Generating deepcopy functions"
	controller-gen object:headerFile="scripts/boilerplate.go.txt" paths="./api/..."
	@echo "▸ Generating CRD manifests"
	controller-gen crd paths="./api/..." output:crd:artifacts:config=manifests/crds

## ── Dashboard ──────────────────────────────────────────────────────────────

dashboard-install:
	@echo "▸ Installing dashboard npm dependencies"
	@cd $(DASHBOARD_DIR) && npm ci

dashboard:
	@echo "▸ Building Next.js dashboard (static export)"
	@cd $(DASHBOARD_DIR) && npm run build

dashboard-dev:
	@echo "▸ Starting Next.js dev server on :3000"
	@cd $(DASHBOARD_DIR) && npm run dev

## ── Docker ─────────────────────────────────────────────────────────────────

docker-build:
	@echo "▸ Building Docker image $(IMAGE):$(VERSION)"
	docker build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

docker-push:
	@echo "▸ Pushing $(IMAGE):$(VERSION)"
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest

## ── Kubernetes ─────────────────────────────────────────────────────────────

# Install all KubePilot CRDs into the current cluster context.
deploy-crds:
	@echo "▸ Installing CRDs"
	kubectl apply -f manifests/crds/

# Remove KubePilot CRDs (non-destructive: leaves CRD instances in place).
undeploy-crds:
	@echo "▸ Removing CRDs"
	kubectl delete -f manifests/crds/ --ignore-not-found

# Create the security namespace for CR code secrets.
deploy-security-ns:
	kubectl create namespace kubepilot-security --dry-run=client -o yaml | kubectl apply -f -

## ── Local development cluster (kind) ────────────────────────────────────────

kind-up:
	@echo "▸ Starting kind cluster kubepilot-dev"
	kind create cluster --name kubepilot-dev --config scripts/kind-config.yaml || true

kind-down:
	kind delete cluster --name kubepilot-dev

## ── Run locally ────────────────────────────────────────────────────────────

# Run the full server locally (requires Ollama running at localhost:11434).
# Override model: KUBEPILOT_OLLAMA_MODEL=mistral make run-server
# Override base URL: KUBEPILOT_OLLAMA_BASE_URL=http://my-ollama:11434/v1 make run-server
run-server: build
	@echo "▸ Starting KubePilot server"
	./$(DIST_DIR)/$(BINARY) serve

run-operator: build
	@echo "▸ Starting KubePilot operator"
	./$(DIST_DIR)/$(BINARY) operator --leader-elect=false

## ── Quality ─────────────────────────────────────────────────────────────────

test:
	@echo "▸ Running tests"
	$(GO) test -race -coverprofile=coverage.txt ./...

lint:
	@echo "▸ Running golangci-lint"
	golangci-lint run ./...

vet:
	$(GO) vet ./...

## ── Clean ───────────────────────────────────────────────────────────────────

clean:
	@rm -rf $(DIST_DIR)
	@rm -rf $(DASHBOARD_DIR)/.next $(DASHBOARD_DIR)/out
	@rm -f coverage.txt

## ── Help ────────────────────────────────────────────────────────────────────

help:
	@echo ""
	@echo "KubePilot – Makefile targets:"
	@echo ""
	@echo "  make              Build dashboard + Go binary (default)"
	@echo "  make build        Build Go binary for host platform"
	@echo "  make build-linux  Cross-compile for linux/amd64 and linux/arm64"
	@echo "  make dashboard    Build Next.js static dashboard"
	@echo "  make dashboard-dev  Start Next.js dev server on :3000"
	@echo "  make generate     Regenerate DeepCopy & CRD manifests"
	@echo "  make docker-build Build Docker image"
	@echo "  make deploy-crds  Install CRDs into current cluster"
	@echo "  make kind-up      Start a local kind dev cluster"
	@echo "  make test         Run Go tests with race detector"
	@echo "  make lint         Run golangci-lint"
	@echo "  make clean        Remove build artifacts"
	@echo ""
