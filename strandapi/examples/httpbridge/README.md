# StrandAPI HTTP Bridge

OpenAI-compatible HTTP REST API that translates standard HTTP requests into StrandAPI protocol messages. This lets browsers, `curl`, and any OpenAI SDK client talk to a Strand inference node.

## Architecture

```
  HTTP Client         HTTP Bridge            StrandAPI Server
  (browser,     -->   (this binary)    -->   (inference node)
   curl, SDK)   <--   port 9000       <--   port 6477
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | Interactive status page with live inference testing |
| `GET` | `/v1/models` | List available models |
| `POST` | `/v1/chat/completions` | Chat inference (JSON or SSE streaming) |
| `POST` | `/v1/completions` | Legacy completions |
| `GET` | `/healthz` | Health check |

## Quick Start

```bash
go run ./strandapi/examples/httpbridge
# Bridge is now running at http://localhost:9000
```

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `STRANDAPI_HTTP_ADDR` | `0.0.0.0:9000` | HTTP listen address |
| `STRANDAPI_CORS_ORIGINS` | `http://localhost:9000` | Comma-separated allowed CORS origins |

## Usage Examples

### Non-streaming request

```bash
curl -s http://localhost:9000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "strand-mock-v1",
    "messages": [{"role": "user", "content": "Hello from Strand!"}],
    "stream": false,
    "max_tokens": 512
  }' | jq .
```

### Streaming request (SSE)

```bash
curl -N http://localhost:9000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "strand-mock-v1",
    "messages": [{"role": "user", "content": "Tell me about the Strand Protocol."}],
    "stream": true,
    "max_tokens": 512
  }'
```

### List models

```bash
curl -s http://localhost:9000/v1/models | jq .
```

### Health check

```bash
curl -s http://localhost:9000/healthz
```

## CORS Configuration

By default, only `http://localhost:9000` is allowed as a cross-origin source. To allow additional origins:

```bash
STRANDAPI_CORS_ORIGINS="http://localhost:3000,https://app.example.com" \
  go run ./strandapi/examples/httpbridge
```

Wildcard (`*`) is intentionally not supported. Set the specific origins your frontend uses.

## Docker

```bash
docker build -t strandapi-httpbridge -f strandapi/Dockerfile .
docker run -p 9000:9000 \
  -e STRANDAPI_CORS_ORIGINS="http://localhost:3000" \
  strandapi-httpbridge
```
