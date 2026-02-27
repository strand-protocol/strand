// Command httpbridge exposes the StrandAPI inference handler as an
// OpenAI-compatible HTTP REST API, so browsers and standard HTTP tools
// can talk to a Strand inference node.
//
// Endpoints:
//
//	GET  /                          — status page (HTML)
//	GET  /v1/models                 — list available models
//	POST /v1/chat/completions       — chat inference (streaming SSE or JSON)
//	POST /v1/completions            — legacy completions
//	GET  /healthz                   — health check
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
	"github.com/strand-protocol/strand/strandapi/pkg/server"
)

// --------------------------------------------------------------------------
// Mock inference handler (same logic as examples/inference)
// --------------------------------------------------------------------------

type streamHandler struct{}

func (h *streamHandler) HandleTokenStream(_ context.Context, req *protocol.InferenceRequest, sender server.TokenSender) error {
	words := strings.Fields(req.Prompt)
	for i, word := range words {
		token := word
		if i > 0 {
			token = " " + word
		}
		if err := sender.Send(&protocol.TokenStreamChunk{
			RequestID: req.ID,
			SeqNum:    uint32(i),
			Token:     token,
			Logprob:   -0.1 * float32(i+1),
		}); err != nil {
			return err
		}
	}
	return nil
}

// --------------------------------------------------------------------------
// OpenAI-compatible types
// --------------------------------------------------------------------------

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	MaxTokens int          `json:"max_tokens"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type sseChoice struct {
	Index int    `json:"index"`
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type sseChunk struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64       `json:"created"`
	Model   string      `json:"model"`
	Choices []sseChoice `json:"choices"`
}

// --------------------------------------------------------------------------
// JSON error responses
// --------------------------------------------------------------------------

// apiError is the structured error format returned by all error paths.
type apiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// writeJSONError writes a structured JSON error response.
func writeJSONError(w http.ResponseWriter, code int, errType, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	e := apiError{}
	e.Error.Message = msg
	e.Error.Type = errType
	e.Error.Code = code
	json.NewEncoder(w).Encode(e)
}

// --------------------------------------------------------------------------
// Metrics
// --------------------------------------------------------------------------

// bridgeMetrics tracks request counts and errors for the /metrics endpoint.
type bridgeMetrics struct {
	requestCount atomic.Int64
	errorCount   atomic.Int64
	streamCount  atomic.Int64
}

var metrics bridgeMetrics

// --------------------------------------------------------------------------
// Handlers
// --------------------------------------------------------------------------

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Strand Protocol — Inference Node</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
           background: #0d1117; color: #e6edf3; min-height: 100vh; padding: 2rem; }
    .container { max-width: 860px; margin: 0 auto; }
    .header { display: flex; align-items: center; gap: 1rem; margin-bottom: 2rem; }
    .logo { font-size: 2rem; font-weight: 700; background: linear-gradient(135deg, #58a6ff, #3fb950);
            -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
    .badge { background: #238636; color: #3fb950; font-size: 0.75rem; padding: 0.2rem 0.6rem;
             border-radius: 9999px; border: 1px solid #3fb950; font-weight: 600; }
    .card { background: #161b22; border: 1px solid #30363d; border-radius: 12px;
            padding: 1.5rem; margin-bottom: 1.5rem; }
    .card h2 { font-size: 1rem; font-weight: 600; color: #58a6ff; margin-bottom: 1rem; }
    .endpoint { display: flex; align-items: baseline; gap: 0.75rem; padding: 0.5rem 0;
                border-bottom: 1px solid #21262d; }
    .endpoint:last-child { border-bottom: none; }
    .method { font-family: monospace; font-size: 0.75rem; font-weight: 700; padding: 0.2rem 0.5rem;
              border-radius: 4px; min-width: 3.5rem; text-align: center; }
    .get  { background: #033a16; color: #3fb950; border: 1px solid #3fb950; }
    .post { background: #0d2d6b; color: #58a6ff; border: 1px solid #58a6ff; }
    .path { font-family: monospace; font-size: 0.9rem; color: #79c0ff; }
    .desc { font-size: 0.85rem; color: #8b949e; margin-left: auto; }
    .try { background: #161b22; border: 1px solid #30363d; border-radius: 12px; padding: 1.5rem; }
    .try h2 { font-size: 1rem; font-weight: 600; color: #58a6ff; margin-bottom: 1rem; }
    textarea { width: 100%; background: #0d1117; border: 1px solid #30363d; border-radius: 8px;
               color: #e6edf3; padding: 0.75rem; font-family: monospace; font-size: 0.9rem;
               resize: vertical; outline: none; }
    textarea:focus { border-color: #58a6ff; }
    button { background: #238636; color: #fff; border: none; border-radius: 8px;
             padding: 0.6rem 1.4rem; font-size: 0.9rem; font-weight: 600; cursor: pointer;
             margin-top: 0.75rem; transition: background 0.15s; }
    button:hover { background: #2ea043; }
    #output { margin-top: 1rem; background: #0d1117; border: 1px solid #30363d; border-radius: 8px;
              padding: 0.75rem; font-family: monospace; font-size: 0.85rem; color: #3fb950;
              min-height: 3rem; white-space: pre-wrap; display: none; }
    .meta { display: flex; gap: 2rem; margin-bottom: 1.5rem; }
    .meta-item { }
    .meta-label { font-size: 0.75rem; color: #8b949e; text-transform: uppercase; letter-spacing: 0.05em; }
    .meta-value { font-size: 1.1rem; font-weight: 600; color: #e6edf3; margin-top: 0.2rem; }
  </style>
</head>
<body>
<div class="container">
  <div class="header">
    <div class="logo">Strand Protocol</div>
    <span class="badge">INFERENCE NODE LIVE</span>
  </div>

  <div class="meta">
    <div class="meta-item">
      <div class="meta-label">Protocol</div>
      <div class="meta-value">StrandAPI v1</div>
    </div>
    <div class="meta-item">
      <div class="meta-label">Transport</div>
      <div class="meta-value">StrandBuf / UDP</div>
    </div>
    <div class="meta-item">
      <div class="meta-label">HTTP Bridge</div>
      <div class="meta-value">OpenAI-compatible</div>
    </div>
    <div class="meta-item">
      <div class="meta-label">Streaming</div>
      <div class="meta-value">SSE</div>
    </div>
  </div>

  <div class="card">
    <h2>API Endpoints</h2>
    <div class="endpoint">
      <span class="method get">GET</span>
      <span class="path">/v1/models</span>
      <span class="desc">List available models</span>
    </div>
    <div class="endpoint">
      <span class="method post">POST</span>
      <span class="path">/v1/chat/completions</span>
      <span class="desc">Chat inference — JSON or SSE streaming</span>
    </div>
    <div class="endpoint">
      <span class="method post">POST</span>
      <span class="path">/v1/completions</span>
      <span class="desc">Legacy completions</span>
    </div>
    <div class="endpoint">
      <span class="method get">GET</span>
      <span class="path">/healthz</span>
      <span class="desc">Health check</span>
    </div>
  </div>

  <div class="try">
    <h2>Try it — live inference</h2>
    <textarea id="prompt" rows="3" placeholder="Enter a prompt...">Hello from the Strand Protocol HTTP bridge! Tell me something interesting.</textarea>
    <div style="display:flex;gap:0.75rem;align-items:center;flex-wrap:wrap">
      <button onclick="runInference(false)">Run (JSON)</button>
      <button onclick="runInference(true)" style="background:#0d2d6b;border:1px solid #58a6ff">Stream (SSE)</button>
    </div>
    <pre id="output"></pre>
  </div>
</div>

<script>
async function runInference(stream) {
  const prompt = document.getElementById('prompt').value.trim();
  const out = document.getElementById('output');
  out.style.display = 'block';
  out.textContent = stream ? '⟳ streaming...\n' : '⟳ waiting...';

  const body = JSON.stringify({
    model: 'strand-mock-v1',
    messages: [{role: 'user', content: prompt}],
    stream,
    max_tokens: 512,
  });

  if (!stream) {
    const r = await fetch('/v1/chat/completions', {method:'POST', headers:{'Content-Type':'application/json'}, body});
    const j = await r.json();
    out.textContent = JSON.stringify(j, null, 2);
    return;
  }

  const r = await fetch('/v1/chat/completions', {method:'POST', headers:{'Content-Type':'application/json'}, body});
  const reader = r.body.getReader();
  const dec = new TextDecoder();
  out.textContent = '';
  while (true) {
    const {done, value} = await reader.read();
    if (done) break;
    const lines = dec.decode(value).split('\n');
    for (const line of lines) {
      if (!line.startsWith('data: ') || line === 'data: [DONE]') continue;
      try {
        const chunk = JSON.parse(line.slice(6));
        const delta = chunk.choices?.[0]?.delta?.content;
        if (delta) out.textContent += delta;
      } catch(_) {}
    }
  }
}
</script>
</body>
</html>`)
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{
				"id":       "strand-mock-v1",
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "strand-protocol",
				"sad":      "sad:llm-inference:mock:128k",
			},
		},
	})
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{
		"request_count": metrics.requestCount.Load(),
		"error_count":   metrics.errorCount.Load(),
		"stream_count":  metrics.streamCount.Load(),
	})
}

// maxAllowedTokens is the upper bound for max_tokens in a request.
const maxAllowedTokens = 100000

func handleChat(sh *streamHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics.requestCount.Add(1)

		if r.Method != http.MethodPost {
			metrics.errorCount.Add(1)
			writeJSONError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed")
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			metrics.errorCount.Add(1)
			writeJSONError(w, http.StatusBadRequest, "invalid_request_error", "read error")
			return
		}

		var req chatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			metrics.errorCount.Add(1)
			writeJSONError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON: "+err.Error())
			return
		}

		// Validate max_tokens.
		if req.MaxTokens <= 0 || req.MaxTokens > maxAllowedTokens {
			metrics.errorCount.Add(1)
			writeJSONError(w, http.StatusBadRequest, "invalid_request_error",
				fmt.Sprintf("max_tokens must be between 1 and %d", maxAllowedTokens))
			return
		}

		// Flatten messages into a single prompt.
		var prompt strings.Builder
		for _, m := range req.Messages {
			if m.Role == "user" {
				if prompt.Len() > 0 {
					prompt.WriteString("\n")
				}
				prompt.WriteString(m.Content)
			}
		}
		if prompt.Len() == 0 {
			metrics.errorCount.Add(1)
			writeJSONError(w, http.StatusBadRequest, "invalid_request_error", "no user message")
			return
		}

		strandReq := &protocol.InferenceRequest{
			Prompt:    prompt.String(),
			MaxTokens: uint32(req.MaxTokens),
			Metadata:  map[string]string{"model": req.Model},
		}

		if req.Stream {
			metrics.streamCount.Add(1)
			// --- SSE streaming ---
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			if !ok {
				metrics.errorCount.Add(1)
				writeJSONError(w, http.StatusInternalServerError, "server_error", "streaming not supported")
				return
			}

			reqID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
			created := time.Now().Unix()

			sender := &sseSender{
				w: w, f: flusher,
				id: reqID, model: req.Model, created: created,
			}
			_ = sh.HandleTokenStream(r.Context(), strandReq, sender)

			// Send [DONE]
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		// --- Blocking response ---
		var sb strings.Builder
		collector := &collectSender{buf: &sb}
		_ = sh.HandleTokenStream(r.Context(), strandReq, collector)

		text := sb.String()
		resp := chatResponse{
			ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []chatChoice{{
				Index:        0,
				Message:      chatMessage{Role: "assistant", Content: text},
				FinishReason: "stop",
			}},
		}
		resp.Usage.PromptTokens = len(strings.Fields(prompt.String()))
		resp.Usage.CompletionTokens = len(strings.Fields(text))
		resp.Usage.TotalTokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// sseSender streams tokens as Server-Sent Events.
type sseSender struct {
	w       http.ResponseWriter
	f       http.Flusher
	id      string
	model   string
	created int64
}

func (s *sseSender) Send(chunk *protocol.TokenStreamChunk) error {
	c := sseChunk{
		ID:      s.id,
		Object:  "chat.completion.chunk",
		Created: s.created,
		Model:   s.model,
		Choices: []sseChoice{{
			Index: 0,
			Delta: struct {
				Content string `json:"content"`
			}{Content: chunk.Token},
		}},
	}
	b, _ := json.Marshal(c)
	fmt.Fprintf(s.w, "data: %s\n\n", b)
	s.f.Flush()
	return nil
}

// collectSender gathers all tokens into a buffer for the blocking response.
type collectSender struct{ buf *strings.Builder }

func (c *collectSender) Send(chunk *protocol.TokenStreamChunk) error {
	c.buf.WriteString(chunk.Token)
	return nil
}

// --------------------------------------------------------------------------
// Middleware
// --------------------------------------------------------------------------

// corsAllowedOrigins parses the STRANDAPI_CORS_ORIGINS env var (comma-separated)
// and returns an origin allowlist. Defaults to "http://localhost:9000".
func corsAllowedOrigins() []string {
	raw := os.Getenv("STRANDAPI_CORS_ORIGINS")
	if raw == "" {
		return []string{"http://localhost:9000"}
	}
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

// corsMiddleware checks the request Origin against the allowlist and sets
// CORS headers only when matched. Never emits a wildcard "*".
func corsMiddleware(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			for _, a := range allowed {
				if a == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					break
				}
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeaders adds defensive HTTP headers to every response.
// CSP allows inline scripts for the interactive status page.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

// requestIDMiddleware generates a unique X-Request-ID for every request.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

// --------------------------------------------------------------------------
// Main
// --------------------------------------------------------------------------

func main() {
	httpAddr := "0.0.0.0:9000"
	if v := os.Getenv("STRANDAPI_HTTP_ADDR"); v != "" {
		httpAddr = v
	}
	if len(os.Args) > 1 {
		httpAddr = os.Args[1]
	}

	sh := &streamHandler{}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/metrics", handleMetrics)
	mux.HandleFunc("/v1/models", handleModels)
	mux.HandleFunc("/v1/chat/completions", handleChat(sh))
	mux.HandleFunc("/v1/completions", handleChat(sh)) // alias

	// Apply middleware: request ID -> security headers -> CORS -> mux
	allowed := corsAllowedOrigins()
	handler := requestIDMiddleware(securityHeaders(corsMiddleware(allowed, mux)))

	srv := &http.Server{
		Addr:         httpAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("StrandAPI HTTP bridge listening on http://%s", httpAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}
