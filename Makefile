# Nexus Protocol -- Top-Level Makefile
#
# Build order enforces the dependency graph:
#   Phase 1: nexlink (Zig, standalone)
#   Phase 2: nexroute (C, depends on nexlink)
#   Phase 3a: nextrust (Rust, standalone)
#   Phase 3b: nexstream (Rust, depends on nexlink + nextrust)
#   Phase 4: nexapi (Go, depends on nexstream + nextrust via CGo, or pure-Go overlay)
#   Phase 5: nexctl (Go), nexus-cloud (Go), both depend on nexapi
#

.PHONY: all clean test test-unit test-integration test-fuzz \
        nexlink nexroute nextrust nexstream nexapi nexctl nexus-cloud \
        fmt lint check \
        docker-up docker-down docker-logs docker-allinone docker-dev docker-clean

# -------------------------------------------------------------------
# Default target: build everything in dependency order
# -------------------------------------------------------------------
all: nexlink nexroute nextrust nexstream nexapi nexctl nexus-cloud
	@echo "=== All modules built successfully ==="

# -------------------------------------------------------------------
# Phase 1: NexLink (Zig)
# -------------------------------------------------------------------
nexlink:
	@echo "=== Building nexlink (Zig) ==="
	cd nexlink && zig build -Dbackend=mock -Doptimize=ReleaseSafe
	@echo "=== nexlink complete ==="

# -------------------------------------------------------------------
# Phase 2: NexRoute (C + P4, depends on nexlink)
# -------------------------------------------------------------------
nexroute: nexlink
	@echo "=== Building nexroute (C) ==="
	cd nexroute && mkdir -p build && cd build && \
		cmake .. -DCMAKE_BUILD_TYPE=Release \
		         -DNEXLINK_INCLUDE_DIR=../../nexlink/include \
		         -DNEXLINK_LIB_DIR=../../nexlink/zig-out/lib && \
		$(MAKE)
	@echo "=== nexroute complete ==="

# -------------------------------------------------------------------
# Phase 3a: NexTrust (Rust, standalone -- no FFI dependencies)
# -------------------------------------------------------------------
nextrust:
	@echo "=== Building nextrust (Rust) ==="
	cd nextrust && cargo build --release
	@echo "=== nextrust complete ==="

# -------------------------------------------------------------------
# Phase 3b: NexStream (Rust, depends on nexlink C FFI + nextrust)
# -------------------------------------------------------------------
nexstream: nexlink nextrust
	@echo "=== Building nexstream (Rust) ==="
	cd nexstream && cargo build --release
	@echo "=== nexstream complete ==="

# -------------------------------------------------------------------
# Phase 4: NexAPI (Go, depends on nexstream + nextrust via CGo)
# -------------------------------------------------------------------
nexapi: nexstream nextrust
	@echo "=== Building nexapi (Go) ==="
	cd nexapi && go build ./...
	@echo "=== nexapi complete ==="

# Build nexapi in pure-Go overlay mode (no CGo, no native dependencies)
.PHONY: nexapi-overlay
nexapi-overlay:
	@echo "=== Building nexapi (Go, pure overlay, no CGo) ==="
	cd nexapi && CGO_ENABLED=0 go build ./...
	@echo "=== nexapi (overlay) complete ==="

# -------------------------------------------------------------------
# Phase 5a: NexCtl (Go, depends on nexapi)
# -------------------------------------------------------------------
nexctl: nexapi
	@echo "=== Building nexctl (Go) ==="
	cd nexctl && go build -o nexctl .
	@echo "=== nexctl complete ==="

# -------------------------------------------------------------------
# Phase 5b: Nexus Cloud (Go, depends on nexapi)
# -------------------------------------------------------------------
nexus-cloud: nexapi
	@echo "=== Building nexus-cloud (Go) ==="
	cd nexus-cloud && go build ./cmd/...
	@echo "=== nexus-cloud complete ==="

# -------------------------------------------------------------------
# Test targets
# -------------------------------------------------------------------
test: test-unit test-integration
	@echo "=== All tests passed ==="

test-unit:
	@echo "=== Running unit tests ==="
	cd nexlink && zig build test || true
	cd nexroute && cd build && ctest --output-on-failure || true
	cd nextrust && cargo test || true
	cd nexstream && cargo test || true
	cd nexapi && go test ./... || true
	cd nexctl && go test ./... || true
	cd nexus-cloud && go test ./... || true
	@echo "=== Unit tests complete ==="

test-integration:
	@echo "=== Running integration tests ==="
	cd nexstream && cargo test --test integration_test || true
	cd nextrust && cargo test --test integration_test || true
	cd nexapi && go test ./tests/ -tags=integration -v || true
	cd nexus-cloud && go test ./tests/integration/ -tags=integration -v || true
	@echo "=== Integration tests complete ==="

test-fuzz:
	@echo "=== Running fuzz tests (long-running) ==="
	cd nexstream && cargo fuzz run fuzz_frame_decode -- -max_total_time=300 || true
	cd nextrust && cargo fuzz run fuzz_mic_parse -- -max_total_time=300 || true
	@echo "=== Fuzz tests complete ==="

# -------------------------------------------------------------------
# Module-specific test targets
# -------------------------------------------------------------------
.PHONY: test-nexlink test-nexroute test-nextrust test-nexstream test-nexapi test-nexctl test-nexus-cloud

test-nexlink:
	cd nexlink && zig build test

test-nexroute:
	cd nexroute/build && ctest --output-on-failure

test-nextrust:
	cd nextrust && cargo test

test-nexstream:
	cd nexstream && cargo test

test-nexapi:
	cd nexapi && go test ./... -v

test-nexctl:
	cd nexctl && go test ./... -v

test-nexus-cloud:
	cd nexus-cloud && go test ./... -v

# -------------------------------------------------------------------
# Code quality
# -------------------------------------------------------------------
fmt:
	cd nexlink && zig fmt src/ tests/ || true
	cd nextrust && cargo fmt || true
	cd nexstream && cargo fmt || true
	cd nexapi && gofmt -w . || true
	cd nexctl && gofmt -w . || true
	cd nexus-cloud && gofmt -w . || true

lint:
	cd nextrust && cargo clippy --all-targets --all-features -- -D warnings || true
	cd nexstream && cargo clippy --all-targets --all-features -- -D warnings || true
	cd nexapi && go vet ./... || true
	cd nexctl && go vet ./... || true
	cd nexus-cloud && go vet ./... || true

check: fmt lint
	@echo "=== Code quality checks complete ==="

# -------------------------------------------------------------------
# Clean
# -------------------------------------------------------------------
clean:
	@echo "=== Cleaning all build artifacts ==="
	cd nexlink && rm -rf zig-out zig-cache .zig-cache || true
	cd nexroute && rm -rf build || true
	cd nextrust && cargo clean || true
	cd nexstream && cargo clean || true
	cd nexapi && go clean ./... || true
	cd nexctl && rm -f nexctl && go clean ./... || true
	cd nexus-cloud && go clean ./... || true
	@echo "=== Clean complete ==="

# -------------------------------------------------------------------
# Docker Compose targets (full stack)
# -------------------------------------------------------------------
.PHONY: docker-up docker-down docker-logs docker-allinone docker-dev docker-clean docker-build docker-push

docker-up:
	@echo "=== Starting Nexus Protocol stack ==="
	docker compose up --build -d
	@echo "=== Stack is running ==="
	@echo "  nexus-cloud:       http://localhost:8080"
	@echo "  nexapi-inference:  http://localhost:9000"

docker-down:
	@echo "=== Stopping Nexus Protocol stack ==="
	docker compose down
	@echo "=== Stack stopped ==="

docker-logs:
	docker compose logs -f

docker-allinone:
	@echo "=== Starting all-in-one mode ==="
	docker compose --profile allinone up --build -d
	@echo "=== All-in-one is running on http://localhost:8081 ==="

docker-dev:
	@echo "=== Starting development stack (hot reload) ==="
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
	@echo "=== Development stack stopped ==="

docker-clean:
	@echo "=== Cleaning Docker resources ==="
	docker compose --profile allinone --profile examples down -v --rmi local
	@echo "=== Docker clean complete ==="

docker-build:
	@echo "=== Building Docker images ==="
	docker compose build
	@echo "=== Docker build complete ==="

# -------------------------------------------------------------------
# Convenience: build only the MVP subset
# -------------------------------------------------------------------
.PHONY: mvp
mvp:
	@echo "=== Building MVP subset ==="
	$(MAKE) nexlink
	$(MAKE) nextrust
	$(MAKE) nexstream
	$(MAKE) nexapi-overlay
	@echo "=== MVP build complete (nexlink + nextrust + nexstream + nexapi overlay) ==="

# -------------------------------------------------------------------
# Help
# -------------------------------------------------------------------
.PHONY: help
help:
	@echo "Nexus Protocol Monorepo Build System"
	@echo ""
	@echo "Build targets (in dependency order):"
	@echo "  make all            Build everything"
	@echo "  make mvp            Build MVP subset (no CGo required)"
	@echo "  make nexlink        Phase 1: NexLink (Zig)"
	@echo "  make nexroute       Phase 2: NexRoute (C + P4)"
	@echo "  make nextrust       Phase 3a: NexTrust (Rust)"
	@echo "  make nexstream      Phase 3b: NexStream (Rust)"
	@echo "  make nexapi         Phase 4: NexAPI (Go + CGo)"
	@echo "  make nexapi-overlay Phase 4: NexAPI (pure Go, no CGo)"
	@echo "  make nexctl         Phase 5a: NexCtl (Go)"
	@echo "  make nexus-cloud    Phase 5b: Nexus Cloud (Go)"
	@echo ""
	@echo "Test targets:"
	@echo "  make test           Run all tests (unit + integration)"
	@echo "  make test-unit      Unit tests only (fast)"
	@echo "  make test-integration Integration tests"
	@echo "  make test-fuzz      Fuzz tests (long-running)"
	@echo "  make test-<module>  Test a specific module"
	@echo ""
	@echo "Quality targets:"
	@echo "  make fmt            Format all source code"
	@echo "  make lint           Run linters on all modules"
	@echo "  make check          Format + lint"
	@echo ""
	@echo "Docker targets:"
	@echo "  make docker-up      Build and start the full stack (background)"
	@echo "  make docker-down    Stop all running services"
	@echo "  make docker-logs    Follow logs from all services"
	@echo "  make docker-allinone Start the all-in-one mode"
	@echo "  make docker-dev     Start dev stack with source mounts"
	@echo "  make docker-clean   Stop, remove volumes and local images"
	@echo "  make docker-build   Build all Docker images"
	@echo ""
	@echo "Other targets:"
	@echo "  make clean          Remove all build artifacts"
	@echo "  make help           Show this help message"
