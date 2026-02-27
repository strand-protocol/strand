# ============================================================
# nexctl -- Module-Level Makefile
# ============================================================

MODULE  := github.com/nexus-protocol/nexus/nexctl
BINARY  := nexctl
LDFLAGS := -s -w

# Cross-compile matrix
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

.PHONY: all build test install cross-compile clean fmt lint

# -----------------------------------------------------------
# Default
# -----------------------------------------------------------
all: build

# -----------------------------------------------------------
# Build targets
# -----------------------------------------------------------
build:
	@echo "=== Building $(BINARY) ==="
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) .
	@echo "=== $(BINARY) built ==="

install:
	@echo "=== Installing $(BINARY) ==="
	CGO_ENABLED=0 go install -ldflags "$(LDFLAGS)" .
	@echo "=== $(BINARY) installed to $$(go env GOPATH)/bin ==="

# -----------------------------------------------------------
# Cross-compile: produces dist/<os>_<arch>/nexctl
# -----------------------------------------------------------
cross-compile:
	@echo "=== Cross-compiling $(BINARY) ==="
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		outdir="dist/$${os}_$${arch}"; \
		mkdir -p "$$outdir"; \
		echo "  -> $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			go build -ldflags "$(LDFLAGS)" \
			-o "$$outdir/$(BINARY)$$ext" . || exit 1; \
	done
	@echo "=== Cross-compilation complete ==="

# -----------------------------------------------------------
# Test targets
# -----------------------------------------------------------
test:
	@echo "=== Running nexctl tests ==="
	go test ./... -v -race -count=1
	@echo "=== Tests complete ==="

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
	rm -f $(BINARY)
	rm -rf dist/
	go clean ./...
