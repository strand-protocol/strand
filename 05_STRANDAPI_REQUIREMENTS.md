# StrandAPI — Layer 5: AI-Native Application Protocol

## Module: `strandapi/`

## Language: Go (1.22+)

## Module Path: `github.com/strand-protocol/strandapi`

---

## 1. Overview

StrandAPI is the application layer of the Strand Protocol stack — the layer that AI developers interact with directly. It replaces HTTP, REST, gRPC, and WebSocket with AI-native primitives: inference requests, token streams, tensor transfers, context sharing, agent negotiation, and tool invocation. StrandAPI operates on top of StrandStream (transport) and StrandTrust (authentication/encryption).

StrandAPI is designed for zero-friction developer adoption. A Go developer should be able to replace an HTTP-based AI API call with a StrandAPI call in under 10 lines of code change, while gaining streaming, semantic routing, and mutual model attestation automatically.

---

## 2. Standards & RFCs Being Replaced / Extended

| Standard | Title | Relevance |
|----------|-------|-----------|
| **RFC 9110** | HTTP Semantics | StrandAPI replaces HTTP request/response semantics with AI-native request types |
| **RFC 9113** | HTTP/2 | Reference for multiplexed streams — StrandAPI multiplexes via StrandStream, not HTTP/2 |
| **RFC 9114** | HTTP/3 | Reference for QUIC-based multiplexing that StrandAPI improves upon via StrandStream |
| **RFC 6455** | The WebSocket Protocol | StrandAPI replaces WebSocket for bidirectional streaming with native stream primitives |
| **RFC 7540** §8 | HTTP/2 Server Push | Reference for server-initiated streams — StrandAPI supports bidirectional stream initiation natively |
| **gRPC Specification** | gRPC over HTTP/2 | StrandAPI replaces gRPC with native AI primitives. Reference for service definition, streaming RPC, and code generation patterns |
| **OpenAPI 3.1** (fka Swagger) | API Description Format | Reference for API definition — StrandAPI defines its own schema language (StrandSchema) optimized for AI service discovery |
| **RFC 8259** | JSON (JavaScript Object Notation) | StrandAPI replaces JSON serialization with a binary format (StrandBuf) for zero-copy tensor and structured data |
| **RFC 8949** | CBOR (Concise Binary Object Representation) | Reference for binary serialization — StrandBuf borrows concepts from CBOR |
| **FlatBuffers** | Google FlatBuffers | Primary reference for StrandBuf's zero-copy serialization design |
| **RFC 7049** | MessagePack | Alternative binary serialization reference |
| **OpenAI API Spec** | OpenAI Chat Completions API | De facto standard for LLM inference APIs. StrandAPI's InferenceRequest/Response are designed to be a superset of this API's functionality |
| **Anthropic Messages API** | Anthropic Claude API | Reference for multi-turn conversation, tool use, and streaming patterns |
| **Server-Sent Events (SSE)** | W3C EventSource | Reference for HTTP-based streaming that StrandAPI replaces with native StrandStream token streaming |

---

## 3. StrandAPI Protocol Specification

### 3.1 Message Types

StrandAPI defines a set of typed messages exchanged over StrandStream streams. Each message has a fixed header followed by a StrandBuf-encoded body.

```
StrandAPI Message Header (16 bytes):
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|         Message Type (16)       |         Flags (16)          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Request ID (32)                            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Body Length (32)                            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Reserved (32)                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 3.2 Message Type Registry

| Type ID | Name | Direction | Stream Mode | Description |
|---------|------|-----------|-------------|-------------|
| `0x0001` | `INFERENCE_REQUEST` | Client → Server | RO | Request model inference (text generation, classification, embedding, etc.) |
| `0x0002` | `INFERENCE_RESPONSE` | Server → Client | RO | Complete inference response (non-streaming) |
| `0x0003` | `TOKEN_STREAM_START` | Server → Client | RO | Begin streaming token response |
| `0x0004` | `TOKEN_STREAM_CHUNK` | Server → Client | RU | Individual token(s) in a stream (unordered OK — client reassembles via sequence) |
| `0x0005` | `TOKEN_STREAM_END` | Server → Client | RO | End of token stream with final metadata |
| `0x0006` | `TENSOR_TRANSFER` | Bidirectional | RU | Bulk tensor data transfer (model weights, activations, gradients) |
| `0x0007` | `CONTEXT_SHARE` | Client → Server | RO | Share conversation context / system prompt for multi-turn |
| `0x0008` | `CONTEXT_ACK` | Server → Client | RO | Acknowledge context received and cached |
| `0x0009` | `TOOL_INVOKE` | Server → Client | RO | Model requests tool execution from client |
| `0x000A` | `TOOL_RESULT` | Client → Server | RO | Client returns tool execution result |
| `0x000B` | `AGENT_NEGOTIATE` | Bidirectional | RO | Agent-to-agent capability negotiation |
| `0x000C` | `AGENT_DELEGATE` | Client → Server | RO | Delegate a sub-task to another agent |
| `0x000D` | `AGENT_RESULT` | Server → Client | RO | Result of delegated sub-task |
| `0x000E` | `HEALTH_CHECK` | Client → Server | BE | Lightweight health/readiness probe |
| `0x000F` | `HEALTH_STATUS` | Server → Client | BE | Health check response |
| `0x0010` | `METRICS_REPORT` | Server → Client | BE | Real-time inference metrics (latency, tokens/sec, queue depth) |
| `0x0011` | `CANCEL` | Client → Server | RO | Cancel an in-flight request |
| `0x0012` | `ERROR` | Bidirectional | RO | Error response with structured error code and details |
| `0x0100` | `CUSTOM` | Bidirectional | Any | Application-defined message type |

### 3.3 Inference Request Schema

```go
type InferenceRequest struct {
    RequestID     uint32              `strandbuf:"1"`
    ModelSelector *SAD                `strandbuf:"2"`  // Optional: semantic routing via StrandRoute
    Messages      []Message           `strandbuf:"3"`  // Conversation history
    SystemPrompt  string              `strandbuf:"4"`  // System instructions
    MaxTokens     uint32              `strandbuf:"5"`
    Temperature   float32             `strandbuf:"6"`
    TopP          float32             `strandbuf:"7"`
    TopK          uint32              `strandbuf:"8"`
    StopSequences []string            `strandbuf:"9"`
    Tools         []ToolDefinition    `strandbuf:"10"` // Available tools for function calling
    Stream        bool                `strandbuf:"11"` // Request streaming response
    Metadata      map[string]string   `strandbuf:"12"` // Custom key-value metadata
    TensorInputs  []TensorRef         `strandbuf:"13"` // References to tensor data (for multimodal)
}

type Message struct {
    Role    string      `strandbuf:"1"`  // "system", "user", "assistant", "tool"
    Content []Content   `strandbuf:"2"`  // Multi-part content (text, image, tensor ref)
    Name    string      `strandbuf:"3"`  // Optional sender name
    ToolUse *ToolUse    `strandbuf:"4"`  // Tool invocation details (for assistant role)
}

type Content struct {
    Type     string    `strandbuf:"1"`  // "text", "image", "tensor_ref"
    Text     string    `strandbuf:"2"`
    ImageRef string    `strandbuf:"3"`  // Reference to image tensor
    TensorID uint32    `strandbuf:"4"`  // Reference to transferred tensor
}

type ToolDefinition struct {
    Name        string `strandbuf:"1"`
    Description string `strandbuf:"2"`
    Parameters  []byte `strandbuf:"3"`  // StrandBuf-encoded parameter schema
}
```

### 3.4 Token Stream Protocol

For streaming inference, the server opens a StrandStream Reliable-Unordered stream for token delivery:

```
Client                                    Server
  |                                          |
  |-- INFERENCE_REQUEST (stream=true) ------>|
  |                                          |
  |<-- TOKEN_STREAM_START -------------------|
  |     stream_id, estimated_tokens          |
  |                                          |
  |<-- TOKEN_STREAM_CHUNK (seq=0) ----------|   (RU mode: may arrive out of order)
  |     tokens: ["Hello"]                    |
  |<-- TOKEN_STREAM_CHUNK (seq=1) ----------|
  |     tokens: [" world"]                   |
  |<-- TOKEN_STREAM_CHUNK (seq=2) ----------|
  |     tokens: ["!"]                        |
  |                                          |
  |<-- TOKEN_STREAM_END --------------------|
  |     total_tokens, latency, cost          |
```

Token chunks include sequence numbers for client-side reassembly. Using RU mode means the server can send tokens as fast as they're generated without waiting for acknowledgment of prior tokens, and a dropped/delayed chunk doesn't block subsequent chunks.

### 3.5 Tensor Transfer Protocol

For bulk tensor data (model weights, activations, embeddings), StrandAPI uses StrandStream with StrandLink's tensor-aware framing:

```go
type TensorTransfer struct {
    TensorID    uint32       `strandbuf:"1"`  // Unique ID for this transfer
    Name        string       `strandbuf:"2"`  // Tensor name (e.g., "layer_0.attention.q_proj.weight")
    Dtype       TensorDtype  `strandbuf:"3"`  // Data type (maps to StrandLink tensor_dtype)
    Shape       []uint32     `strandbuf:"4"`  // Tensor dimensions
    TotalBytes  uint64       `strandbuf:"5"`  // Total size in bytes
    Compression string       `strandbuf:"6"`  // "none", "lz4", "zstd"
    Checksum    [32]byte     `strandbuf:"7"`  // SHA-256 of uncompressed tensor data
    // Payload follows as raw bytes on the StrandStream stream
}
```

Tensor data is sent on a dedicated StrandStream Reliable-Unordered stream with StrandLink's `tensor_payload` flag set, enabling NIC-level memory alignment and optional GPUDirect RDMA delivery.

---

## 4. StrandBuf Serialization Format

### 4.1 Overview

StrandBuf is a FlatBuffers-inspired zero-copy binary serialization format designed for StrandAPI messages. Unlike JSON (RFC 8259) or Protobuf, StrandBuf allows reading fields directly from the wire buffer without deserialization.

### 4.2 Encoding Rules

- Fixed-size fields (integers, floats, booleans) stored inline at fixed offsets
- Variable-size fields (strings, byte arrays, nested structs) stored via offset table
- Field numbering is explicit (not positional) for backward/forward compatibility
- Unknown fields are skipped (forward compatibility)
- No self-describing schema — both sides must agree on schema version

### 4.3 Code Generation

StrandBuf schemas defined in `.strandbuf` files generate Go structs with marshal/unmarshal methods:

```
// inference.strandbuf
table InferenceRequest {
  request_id: uint32 (id: 1);
  model_selector: SAD (id: 2);
  messages: [Message] (id: 3);
  system_prompt: string (id: 4);
  max_tokens: uint32 (id: 5);
  // ...
}
```

```bash
strandbuf-gen --go --out pkg/protocol/ schemas/inference.strandbuf
```

---

## 5. Architecture & Components

### 5.1 Source Tree Structure

```
strandapi/
├── go.mod
├── go.sum
├── pkg/
│   ├── client/
│   │   ├── client.go               # High-level StrandAPI client
│   │   ├── inference.go             # Inference request/response helpers
│   │   ├── streaming.go             # Token stream consumer
│   │   ├── tensor.go                # Tensor transfer client-side
│   │   ├── tools.go                 # Tool invocation handler (client-side)
│   │   ├── agent.go                 # Agent-to-agent communication
│   │   └── options.go               # Client configuration options
│   ├── server/
│   │   ├── server.go                # High-level StrandAPI server
│   │   ├── handler.go               # Request handler interface
│   │   ├── inference_handler.go     # Inference request dispatcher
│   │   ├── streaming_handler.go     # Token stream producer
│   │   ├── tensor_handler.go        # Tensor transfer server-side
│   │   ├── tools_handler.go         # Tool invocation server-side
│   │   ├── middleware.go            # Middleware chain (logging, metrics, auth, rate limiting)
│   │   └── router.go               # Message type router
│   ├── protocol/
│   │   ├── message.go               # StrandAPI message header encode/decode
│   │   ├── types.go                 # Message type constants and flag definitions
│   │   ├── inference.go             # InferenceRequest/Response structs
│   │   ├── token_stream.go          # TokenStreamStart/Chunk/End structs
│   │   ├── tensor.go                # TensorTransfer struct
│   │   ├── context.go               # ContextShare/ContextAck structs
│   │   ├── tool.go                  # ToolInvoke/ToolResult structs
│   │   ├── agent.go                 # AgentNegotiate/Delegate/Result structs
│   │   ├── error.go                 # Structured error codes and Error message
│   │   └── health.go                # HealthCheck/HealthStatus structs
│   ├── strandbuf/
│   │   ├── encoder.go               # StrandBuf binary encoder
│   │   ├── decoder.go               # StrandBuf binary decoder (zero-copy)
│   │   ├── schema.go                # Runtime schema representation
│   │   └── builder.go               # StrandBuf message builder
│   ├── sad/
│   │   ├── sad.go                   # Semantic Address Descriptor (Go types)
│   │   ├── builder.go               # SAD builder for constructing queries
│   │   └── encode.go                # SAD binary encoding (matches StrandRoute C format)
│   ├── transport/
│   │   ├── adapter.go               # StrandStream Go adapter (CGo FFI to Rust StrandStream)
│   │   ├── connection_pool.go       # Connection pooling and management
│   │   └── overlay_transport.go     # Pure Go overlay transport (UDP/IP, no StrandLink dependency)
│   └── observability/
│       ├── metrics.go               # Prometheus-compatible metrics
│       ├── tracing.go               # Distributed tracing (OpenTelemetry)
│       └── logging.go               # Structured logging
├── cmd/
│   └── strandapi-codegen/
│       └── main.go                  # StrandBuf schema → Go code generator
├── schemas/
│   ├── inference.strandbuf             # Inference request/response schema
│   ├── tensor.strandbuf                # Tensor transfer schema
│   ├── tool.strandbuf                  # Tool invocation schema
│   ├── agent.strandbuf                 # Agent protocol schema
│   └── common.strandbuf                # Shared types (SAD, Content, etc.)
├── tests/
│   ├── client_test.go               # Client API tests
│   ├── server_test.go               # Server handler tests
│   ├── protocol_test.go             # Message encode/decode tests
│   ├── strandbuf_test.go               # StrandBuf encoder/decoder tests
│   ├── streaming_test.go            # Token streaming end-to-end
│   ├── tensor_test.go               # Tensor transfer end-to-end
│   ├── agent_test.go                # Agent negotiation/delegation tests
│   └── integration_test.go          # Full stack integration (client → server → response)
├── benches/
│   ├── strandbuf_bench_test.go         # StrandBuf vs JSON vs Protobuf benchmarks
│   ├── inference_bench_test.go      # Inference request latency
│   └── streaming_bench_test.go      # Token streaming throughput
└── examples/
    ├── simple_inference/
    │   └── main.go                  # Minimal inference request example
    ├── streaming_chat/
    │   └── main.go                  # Streaming chat example
    ├── tensor_sync/
    │   └── main.go                  # Model weight synchronization example
    ├── multi_agent/
    │   └── main.go                  # Multi-agent delegation example
    └── http_bridge/
        └── main.go                  # HTTP ↔ StrandAPI bridge for backward compatibility
```

---

## 6. Functional Requirements

### 6.1 Client SDK

| ID | Requirement | Priority |
|----|-------------|----------|
| NA-CL-001 | `client.Infer(ctx, req)` — send inference request, return complete response | P0 |
| NA-CL-002 | `client.InferStream(ctx, req)` — send inference request, return `TokenStream` iterator | P0 |
| NA-CL-003 | `client.TransferTensor(ctx, tensor)` — send tensor data, return transfer receipt | P0 |
| NA-CL-004 | `client.ReceiveTensor(ctx, tensorID)` — receive tensor data from remote | P0 |
| NA-CL-005 | `client.RegisterTools(tools)` — register client-side tool implementations for model tool use | P1 |
| NA-CL-006 | `client.NegotiateWith(ctx, peerSAD)` — initiate agent-to-agent capability negotiation | P1 |
| NA-CL-007 | `client.DelegateTask(ctx, peerSAD, task)` — delegate sub-task to another agent | P1 |
| NA-CL-008 | `client.Cancel(ctx, requestID)` — cancel in-flight request | P0 |
| NA-CL-009 | Connection pooling with automatic reconnection and health checking | P0 |
| NA-CL-010 | Automatic retry with exponential backoff for transient errors | P1 |
| NA-CL-011 | Client-side SAD construction: `sad.Builder().WithCapability(CodeGen).WithMaxLatency(200).Build()` | P0 |
| NA-CL-012 | Pure Go overlay transport mode (no CGo, no StrandLink/StrandStream dependency) for easy adoption | P0 |

### 6.2 Server SDK

| ID | Requirement | Priority |
|----|-------------|----------|
| NA-SV-001 | `server.HandleInference(handler)` — register inference request handler | P0 |
| NA-SV-002 | `server.HandleStreamingInference(handler)` — register streaming handler that yields tokens | P0 |
| NA-SV-003 | `server.HandleTensorTransfer(handler)` — register tensor receive handler | P0 |
| NA-SV-004 | Middleware support: logging, metrics, authentication, rate limiting, request tracing | P0 |
| NA-SV-005 | Graceful shutdown: drain in-flight requests, close connections cleanly | P0 |
| NA-SV-006 | Health check endpoint: respond to `HEALTH_CHECK` with configurable readiness logic | P0 |
| NA-SV-007 | Automatic capability advertisement: server registers its capabilities with StrandRoute on startup | P1 |
| NA-SV-008 | Connection limit and request rate limiting configurable per-server | P0 |

### 6.3 StrandBuf Serialization

| ID | Requirement | Priority |
|----|-------------|----------|
| NA-NB-001 | Encode Go structs to StrandBuf binary format with struct tags (`strandbuf:"field_id"`) | P0 |
| NA-NB-002 | Decode StrandBuf binary to Go structs with zero-copy for byte slices and strings | P0 |
| NA-NB-003 | Forward compatibility: unknown fields silently skipped on decode | P0 |
| NA-NB-004 | Backward compatibility: missing fields get zero values on decode | P0 |
| NA-NB-005 | Code generator: `.strandbuf` schema files → Go structs with marshal/unmarshal | P1 |
| NA-NB-006 | Performance target: encode/decode at least 3x faster than JSON, comparable to Protobuf | P0 |

### 6.4 HTTP Bridge (Backward Compatibility)

| ID | Requirement | Priority |
|----|-------------|----------|
| NA-BR-001 | HTTP → StrandAPI bridge: accept OpenAI-compatible REST API requests, translate to StrandAPI InferenceRequest | P1 |
| NA-BR-002 | StrandAPI → HTTP bridge: translate StrandAPI responses to JSON HTTP responses | P1 |
| NA-BR-003 | SSE → TokenStream bridge: translate streaming HTTP SSE to StrandAPI TokenStream and vice versa | P1 |
| NA-BR-004 | Bridge runs as a standalone Go binary or importable library | P1 |

---

## 7. Error Codes

| Code | Name | Description |
|------|------|-------------|
| `0x0000` | `OK` | Success |
| `0x0001` | `CANCELLED` | Request cancelled by client |
| `0x0002` | `TIMEOUT` | Request exceeded deadline |
| `0x0003` | `INVALID_REQUEST` | Malformed request |
| `0x0004` | `MODEL_NOT_FOUND` | No model matching SAD found |
| `0x0005` | `MODEL_OVERLOADED` | Target model at capacity |
| `0x0006` | `CONTEXT_TOO_LARGE` | Context exceeds model's window |
| `0x0007` | `TENSOR_MISMATCH` | Tensor shape/dtype mismatch |
| `0x0008` | `TOOL_EXECUTION_FAILED` | Client-side tool execution failed |
| `0x0009` | `TRUST_FAILURE` | StrandTrust handshake or attestation failed |
| `0x000A` | `RATE_LIMITED` | Request rate exceeded |
| `0x000B` | `INTERNAL_ERROR` | Server internal error |
| `0x000C` | `NOT_IMPLEMENTED` | Message type not supported by server |
| `0x00FF` | `CUSTOM` | Application-defined error with detail string |

---

## 8. Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NA-NF-001 | Inference request encode + send latency | < 100μs (StrandBuf encode + StrandStream send) |
| NA-NF-002 | Token stream per-token latency overhead | < 50μs per chunk (StrandAPI overhead only, excludes inference time) |
| NA-NF-003 | StrandBuf encode/decode throughput | > 1 GB/s for typical inference messages |
| NA-NF-004 | Connection pool overhead | < 1ms to acquire connection from pool |
| NA-NF-005 | Memory per active inference stream | < 16KB (excluding tensor data) |
| NA-NF-006 | Concurrent inference requests per server | > 10,000 |
| NA-NF-007 | Pure Go overlay mode (no CGo) | Full functionality, 15-20% higher latency vs native |

---

## 9. Build & Compilation

```bash
# Build all packages
go build ./...

# Run tests
go test ./... -v

# Run benchmarks
go test ./benches/ -bench=. -benchmem

# Build HTTP bridge binary
go build -o strandapi-bridge ./examples/http_bridge/

# Build codegen tool
go build -o strandbuf-gen ./cmd/strandapi-codegen/

# Generate code from schemas
./strandbuf-gen --go --out pkg/protocol/ schemas/*.strandbuf

# Run with race detector
go test -race ./...
```

---

## 10. Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| Go standard library | 1.22+ | Core (net, sync, context, crypto) |
| `golang.org/x/sync` | Latest | `errgroup`, `semaphore` for concurrency |
| `go.opentelemetry.io/otel` | 1.24+ | Distributed tracing |
| `github.com/prometheus/client_golang` | 1.18+ | Prometheus metrics |
| `go.uber.org/zap` | 1.27+ | Structured logging |
| `google.golang.org/protobuf` | — | NOT used — StrandBuf replaces Protobuf |
| StrandStream Rust FFI (CGo) | — | Native transport (optional, via CGo) |
| No CGo dependencies for overlay mode | — | Pure Go overlay transport |

---

## 11. Example: Minimal Inference Client

```go
package main

import (
    "context"
    "fmt"
    "github.com/strand-protocol/strandapi/pkg/client"
    "github.com/strand-protocol/strandapi/pkg/protocol"
    "github.com/strand-protocol/strandapi/pkg/sad"
)

func main() {
    // Connect using overlay transport (pure Go, works anywhere)
    c, err := client.NewOverlay("udp://inference.example.com:6477")
    if err != nil {
        panic(err)
    }
    defer c.Close()

    // Build a semantic address: "any code-gen model with 128K context, under 200ms"
    selector := sad.Builder().
        WithCapability(sad.TextGen | sad.CodeGen).
        WithMinContext(131072).
        WithMaxLatency(200).
        Build()

    // Send streaming inference request
    stream, err := c.InferStream(context.Background(), &protocol.InferenceRequest{
        ModelSelector: selector,
        Messages: []protocol.Message{
            {Role: "user", Content: []protocol.Content{{Type: "text", Text: "Write a quicksort in Rust"}}},
        },
        MaxTokens:   4096,
        Temperature: 0.7,
        Stream:      true,
    })
    if err != nil {
        panic(err)
    }

    // Consume token stream
    for token := range stream.Tokens() {
        fmt.Print(token.Text)
    }
    fmt.Println()

    // Print final metadata
    meta := stream.Metadata()
    fmt.Printf("\nTokens: %d, Latency: %dms\n", meta.TotalTokens, meta.LatencyMs)
}
```
