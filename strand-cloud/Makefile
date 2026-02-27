# ============================================================
# nexus-cloud -- Module-Level Makefile
# ============================================================

MODULE   := github.com/nexus-protocol/nexus/nexus-cloud
BINARY   := nexus-cloud
ALLINONE := nexus-allinone
IMAGE    := nexus-cloud
TAG      ?= latest
REGISTRY ?= ghcr.io/nexus-protocol

LDFLAGS  := -s -w

.PHONY: all build build-allinone test test-unit test-integration \
        docker-build docker-push clean fmt lint

# -----------------------------------------------------------
# Default
# -----------------------------------------------------------
all: build build-allinone

# -----------------------------------------------------------
# Build targets
# -----------------------------------------------------------
build:
	@echo "=== Building $(BINARY) ==="
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/nexus-cloud
	@echo "=== $(BINARY) built ==="

build-allinone:
	@echo "=== Building $(ALLINONE) ==="
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(ALLINONE) ./cmd/nexus-allinone
	@echo "=== $(ALLINONE) built ==="

# -----------------------------------------------------------
# Test targets
# -----------------------------------------------------------
test: test-unit test-integration

test-unit:
	@echo "=== Running unit tests ==="
	go test ./pkg/... ./cmd/... -v -race -count=1
	@echo "=== Unit tests complete ==="

test-integration:
	@echo "=== Running integration tests ==="
	go test ./tests/integration/ -tags=integration -v -race -count=1
	@echo "=== Integration tests complete ==="

# -----------------------------------------------------------
# Docker targets
# -----------------------------------------------------------
docker-build:
	@echo "=== Building Docker image $(IMAGE):$(TAG) ==="
	docker build -t $(REGISTRY)/$(IMAGE):$(TAG) -f Dockerfile .
	docker build -t $(REGISTRY)/$(ALLINONE):$(TAG) -f Dockerfile.allinone .
	@echo "=== Docker build complete ==="

docker-push:
	@echo "=== Pushing Docker images ==="
	docker push $(REGISTRY)/$(IMAGE):$(TAG)
	docker push $(REGISTRY)/$(ALLINONE):$(TAG)
	@echo "=== Docker push complete ==="

# -----------------------------------------------------------
# Code quality
# -----------------------------------------------------------
fmt:
	gofmt -w .

lint:
	go vet ./...

# -----------------------------------------------------------
# Clean
# -----------------------------------------------------------
clean:
	rm -f $(BINARY) $(ALLINONE)
	go clean ./...
