# ============================================================
# nexus-cloud -- Multi-stage Docker build
# Produces the nexus-cloud control-plane binary.
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

# Build the main nexus-cloud binary (static, stripped).
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w" \
    -o /nexus-cloud \
    ./cmd/nexus-cloud

# Also build the all-in-one binary so it is available as a
# bonus artefact; the primary entrypoint remains nexus-cloud.
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w" \
    -o /nexus-allinone \
    ./cmd/nexus-allinone

# Build a tiny static healthcheck binary for use in distroless.
RUN printf 'package main\nimport ("fmt";"net/http";"os")\nfunc main(){r,e:=http.Get("http://localhost:8080/healthz");if e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)};if r.StatusCode!=200{fmt.Fprintf(os.Stderr,"status %%d\\n",r.StatusCode);os.Exit(1)}}' > /tmp/healthcheck.go && \
    CGO_ENABLED=0 go build -ldflags "-s -w" -o /healthcheck /tmp/healthcheck.go

# -----------------------------------------------------------
# Stage 2: Minimal runtime
# -----------------------------------------------------------
FROM gcr.io/distroless/static-debian12

COPY --from=builder /nexus-cloud /nexus-cloud
COPY --from=builder /nexus-allinone /nexus-allinone
COPY --from=builder /healthcheck /healthcheck

EXPOSE 8080

ENTRYPOINT ["/nexus-cloud"]
