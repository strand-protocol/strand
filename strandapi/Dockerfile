# ============================================================
# strandapi -- Multi-stage Docker build
# Produces the StrandAPI inference and echo example binaries.
# ============================================================

# -----------------------------------------------------------
# Stage 1: Builder
# -----------------------------------------------------------
FROM golang:1.26-bookworm AS builder

WORKDIR /src

# Cache dependency downloads in a separate layer.
COPY go.mod ./
# go.sum may not exist if there are no external dependencies.
COPY go.su[m] ./
RUN go mod download

# Copy the full module source.
COPY . .

# Build the inference example server (static, stripped).
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w" \
    -o /strandapi-inference \
    ./examples/inference

# Build the echo example server (static, stripped).
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w" \
    -o /strandapi-echo \
    ./examples/echo

# Build the HTTP bridge (OpenAI-compatible REST frontend).
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w" \
    -o /strandapi-httpbridge \
    ./examples/httpbridge

# -----------------------------------------------------------
# Stage 2: Minimal runtime
# -----------------------------------------------------------
FROM gcr.io/distroless/static-debian12

COPY --from=builder /strandapi-inference /strandapi-inference
COPY --from=builder /strandapi-echo /strandapi-echo
COPY --from=builder /strandapi-httpbridge /strandapi-httpbridge

EXPOSE 9000
EXPOSE 9001

ENTRYPOINT ["/strandapi-inference"]
