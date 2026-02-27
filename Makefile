# Strand Protocol -- Top-Level Makefile
#
# Build order enforces the dependency graph:
#   Phase 1: strandlink (Zig, standalone)
#   Phase 2: strandroute (C, depends on strandlink)
#   Phase 3a: strandtrust (Rust, standalone)
#   Phase 3b: strandstream (Rust, depends on strandlink + strandtrust)
#   Phase 4: strandapi (Go, depends on strandstream + strandtrust via CGo, or pure-Go overlay)
#   Phase 5: strandctl (Go), strand-cloud (Go), both depend on strandapi
#

.PHONY: all clean test test-unit test-integration test-fuzz \
        strandlink strandroute strandtrust strandstream strandapi strandctl strand-cloud \
        fmt lint check \
        docker-up docker-down docker-logs docker-allinone docker-dev docker-clean

# -------------------------------------------------------------------
# Default target: build everything in dependency order
# -------------------------------------------------------------------
all: strandlink strandroute strandtrust strandstream strandapi strandctl strand-cloud
	@echo "=== All modules built successfully ==="

# -------------------------------------------------------------------
# Phase 1: StrandLink (Zig)
# -------------------------------------------------------------------
strandlink:
	@echo "=== Building strandlink (Zig) ==="
	cd strandlink && zig build -Dbackend=mock -Doptimize=ReleaseSafe
	@echo "=== strandlink complete ==="

# -------------------------------------------------------------------
# Phase 2: StrandRoute (C + P4, depends on strandlink)
# -------------------------------------------------------------------
strandroute: strandlink
	@echo "=== Building strandroute (C) ==="
	cd strandroute && mkdir -p build && cd build && \
		cmake .. -DCMAKE_BUILD_TYPE=Release \
		         -DSTRANDLINK_INCLUDE_DIR=../../strandlink/include \
		         -DSTRANDLINK_LIB_DIR=../../strandlink/zig-out/lib && \
		$(MAKE)
	@echo "=== strandroute complete ==="

# -------------------------------------------------------------------
# Phase 3a: StrandTrust (Rust, standalone -- no FFI dependencies)
# -------------------------------------------------------------------
strandtrust:
	@echo "=== Building strandtrust (Rust) ==="
	cd strandtrust && cargo build --release
	@echo "=== strandtrust complete ==="

# -------------------------------------------------------------------
# Phase 3b: StrandStream (Rust, depends on strandlink C FFI + strandtrust)
# -------------------------------------------------------------------
strandstream: strandlink strandtrust
	@echo "=== Building strandstream (Rust) ==="
	cd strandstream && cargo build --release
	@echo "=== strandstream complete ==="

# -------------------------------------------------------------------
# Phase 4: StrandAPI (Go, depends on strandstream + strandtrust via CGo)
# -------------------------------------------------------------------
strandapi: strandstream strandtrust
	@echo "=== Building strandapi (Go) ==="
	cd strandapi && go build ./...
	@echo "=== strandapi complete ==="

# Build strandapi in pure-Go overlay mode (no CGo, no native dependencies)
.PHONY: strandapi-overlay
strandapi-overlay:
	@echo "=== Building strandapi (Go, pure overlay, no CGo) ==="
	cd strandapi && CGO_ENABLED=0 go build ./...
	@echo "=== strandapi (overlay) complete ==="

# -------------------------------------------------------------------
# Phase 5a: StrandCtl (Go, depends on strandapi)
# -------------------------------------------------------------------
strandctl: strandapi
	@echo "=== Building strandctl (Go) ==="
	cd strandctl && go build -o strandctl .
	@echo "=== strandctl complete ==="

# -------------------------------------------------------------------
# Phase 5b: Strand Cloud (Go, depends on strandapi)
# -------------------------------------------------------------------
strand-cloud: strandapi
	@echo "=== Building strand-cloud (Go) ==="
	cd strand-cloud && go build ./cmd/...
	@echo "=== strand-cloud complete ==="

# -------------------------------------------------------------------
# Test targets
# -------------------------------------------------------------------
test: test-unit test-integration
	@echo "=== All tests passed ==="

test-unit:
	@echo "=== Running unit tests ==="
	cd strandlink && zig build test || true
	cd strandroute && cd build && ctest --output-on-failure || true
	cd strandtrust && cargo test || true
	cd strandstream && cargo test || true
	cd strandapi && go test ./... || true
	cd strandctl && go test ./... || true
	cd strand-cloud && go test ./... || true
	@echo "=== Unit tests complete ==="

test-integration:
	@echo "=== Running integration tests ==="
	cd strandstream && cargo test --test integration_test || true
	cd strandtrust && cargo test --test integration_test || true
	cd strandapi && go test ./tests/ -tags=integration -v || true
	cd strand-cloud && go test ./tests/integration/ -tags=integration -v || true
	@echo "=== Integration tests complete ==="

test-fuzz:
	@echo "=== Running fuzz tests (long-running) ==="
	cd strandstream && cargo fuzz run fuzz_frame_decode -- -max_total_time=300 || true
	cd strandtrust && cargo fuzz run fuzz_mic_parse -- -max_total_time=300 || true
	cd tests/fuzz && go test -fuzz=FuzzStrandBufRoundtrip -fuzztime=60s || true
	cd tests/fuzz && go test -fuzz=FuzzFrameRead -fuzztime=60s || true
	cd tests/fuzz && go test -fuzz=FuzzSADParse -fuzztime=60s || true
	cd tests/fuzz && go test -fuzz=FuzzAgentNegotiateDecode -fuzztime=60s || true
	cd tests/fuzz && go test -fuzz=FuzzAgentDelegateDecode -fuzztime=60s || true
	cd tests/fuzz && go test -fuzz=FuzzAgentResultDecode -fuzztime=60s || true
	@echo "=== Fuzz tests complete ==="

# -------------------------------------------------------------------
# Module-specific test targets
# -------------------------------------------------------------------
.PHONY: test-strandlink test-strandroute test-strandtrust test-strandstream test-strandapi test-strandctl test-strand-cloud

test-strandlink:
	cd strandlink && zig build test

test-strandroute:
	cd strandroute/build && ctest --output-on-failure

test-strandtrust:
	cd strandtrust && cargo test

test-strandstream:
	cd strandstream && cargo test

test-strandapi:
	cd strandapi && go test ./... -v

test-strandctl:
	cd strandctl && go test ./... -v

test-strand-cloud:
	cd strand-cloud && go test ./... -v

# -------------------------------------------------------------------
# Code quality
# -------------------------------------------------------------------
fmt:
	cd strandlink && zig fmt src/ tests/ || true
	cd strandtrust && cargo fmt || true
	cd strandstream && cargo fmt || true
	cd strandapi && gofmt -w . || true
	cd strandctl && gofmt -w . || true
	cd strand-cloud && gofmt -w . || true

lint:
	cd strandtrust && cargo clippy --all-targets --all-features -- -D warnings || true
	cd strandstream && cargo clippy --all-targets --all-features -- -D warnings || true
	cd strandapi && go vet ./... || true
	cd strandctl && go vet ./... || true
	cd strand-cloud && go vet ./... || true

check: fmt lint
	@echo "=== Code quality checks complete ==="

# -------------------------------------------------------------------
# Clean
# -------------------------------------------------------------------
clean:
	@echo "=== Cleaning all build artifacts ==="
	cd strandlink && rm -rf zig-out zig-cache .zig-cache || true
	cd strandroute && rm -rf build || true
	cd strandtrust && cargo clean || true
	cd strandstream && cargo clean || true
	cd strandapi && go clean ./... || true
	cd strandctl && rm -f strandctl && go clean ./... || true
	cd strand-cloud && go clean ./... || true
	@echo "=== Clean complete ==="

# -------------------------------------------------------------------
# Docker Compose targets (full stack)
# -------------------------------------------------------------------
.PHONY: docker-up docker-down docker-logs docker-allinone docker-dev docker-clean docker-build docker-push

docker-up:
	@echo "=== Starting Strand Protocol stack ==="
	docker compose up --build -d
	@echo "=== Stack is running ==="
	@echo "  strand-cloud:        http://localhost:8080"
	@echo "  strandapi-inference: http://localhost:9000"

docker-down:
	@echo "=== Stopping Strand Protocol stack ==="
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
	$(MAKE) strandlink
	$(MAKE) strandtrust
	$(MAKE) strandstream
	$(MAKE) strandapi-overlay
	@echo "=== MVP build complete (strandlink + strandtrust + strandstream + strandapi overlay) ==="

# -------------------------------------------------------------------
# Help
# -------------------------------------------------------------------
.PHONY: help
help:
	@echo "Strand Protocol Monorepo Build System"
	@echo ""
	@echo "Build targets (in dependency order):"
	@echo "  make all              Build everything"
	@echo "  make mvp              Build MVP subset (no CGo required)"
	@echo "  make strandlink        Phase 1: StrandLink (Zig)"
	@echo "  make strandroute       Phase 2: StrandRoute (C + P4)"
	@echo "  make strandtrust       Phase 3a: StrandTrust (Rust)"
	@echo "  make strandstream      Phase 3b: StrandStream (Rust)"
	@echo "  make strandapi         Phase 4: StrandAPI (Go + CGo)"
	@echo "  make strandapi-overlay Phase 4: StrandAPI (pure Go, no CGo)"
	@echo "  make strandctl         Phase 5a: StrandCtl (Go)"
	@echo "  make strand-cloud      Phase 5b: Strand Cloud (Go)"
	@echo ""
	@echo "Test targets:"
	@echo "  make test             Run all tests (unit + integration)"
	@echo "  make test-unit        Unit tests only (fast)"
	@echo "  make test-integration Integration tests"
	@echo "  make test-fuzz        Fuzz tests (long-running)"
	@echo "  make test-<module>    Test a specific module"
	@echo ""
	@echo "Quality targets:"
	@echo "  make fmt              Format all source code"
	@echo "  make lint             Run linters on all modules"
	@echo "  make check            Format + lint"
	@echo ""
	@echo "Docker targets:"
	@echo "  make docker-up        Build and start the full stack (background)"
	@echo "  make docker-down      Stop all running services"
	@echo "  make docker-logs      Follow logs from all services"
	@echo "  make docker-allinone  Start the all-in-one mode"
	@echo "  make docker-dev       Start dev stack with source mounts"
	@echo "  make docker-clean     Stop, remove volumes and local images"
	@echo "  make docker-build     Build all Docker images"
	@echo ""
	@echo "Other targets:"
	@echo "  make clean            Remove all build artifacts"
	@echo "  make help             Show this help message"
