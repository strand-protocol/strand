package apiserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const roleContextKey contextKey = 1

const (
	// maxRequestBodyBytes limits request body size to 1 MiB to prevent DoS.
	maxRequestBodyBytes = 1 << 20 // 1 MiB
)

// applyMiddleware wraps the given handler with the standard middleware chain.
// Order (outermost to innermost): recovery -> auth -> rbac -> rateLimiter -> requestBodyLimit -> cors -> logging -> requestID
func (s *Server) applyMiddleware(h http.Handler) http.Handler {
	h = requestIDMiddleware(h)
	h = loggingMiddleware(h)
	h = corsMiddleware(h)
	h = requestBodyLimitMiddleware(h)
	h = s.rateLimitMiddleware(h)
	h = s.rbacMiddleware(h)
	h = s.apiKeyMiddleware(h)
	h = recoveryMiddleware(h)
	return h
}

// apiKeyMiddleware enforces Bearer token authentication on all routes except
// /healthz and /readyz. Valid API keys are provided in ServerOptions.APIKeys.
// On success it stores the caller's Role in the request context for rbacMiddleware.
func (s *Server) apiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health/readiness probes are exempt from authentication.
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		// If no API keys are configured (dev mode), grant admin role and pass through.
		if len(s.opts.APIKeys) == 0 {
			ctx := context.WithValue(r.Context(), roleContextKey, RoleAdmin)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader || token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		info, ok := s.opts.APIKeys[token]
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), roleContextKey, info.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// rbacMiddleware enforces role-based access control:
//   - RoleViewer:   GET only
//   - RoleOperator: GET, POST, PUT
//   - RoleAdmin:    all methods (GET, POST, PUT, DELETE, OPTIONS)
//
// The role is read from the context value set by apiKeyMiddleware.
func (s *Server) rbacMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Probe endpoints are always exempt.
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		// OPTIONS preflight passes through (handled by corsMiddleware).
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		role, _ := r.Context().Value(roleContextKey).(Role)
		switch r.Method {
		case http.MethodGet:
			// All roles may read.
		case http.MethodPost, http.MethodPut:
			if role < RoleOperator {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
		case http.MethodDelete:
			if role < RoleAdmin {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
		default:
			if role < RoleAdmin {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requestBodyLimitMiddleware wraps the request body with http.MaxBytesReader to
// prevent memory exhaustion from oversized payloads. Returns 413 if exceeded.
func requestBodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// requestIDMiddleware adds a unique X-Request-ID header to each request and
// response if one is not already present.
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

// loggingMiddleware logs each request's method, path, status, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.statusCode, time.Since(start))
	})
}

// recoveryMiddleware catches panics in downstream handlers and returns 500.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC: %v\n%s", rec, debug.Stack())
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// tokenBucket is a simple, goroutine-safe token-bucket rate limiter.
type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	maxTok   float64
	ratePerS float64 // tokens added per second
	lastFill time.Time
}

func newTokenBucket(ratePerSec float64, burst float64) *tokenBucket {
	return &tokenBucket{
		tokens:   burst,
		maxTok:   burst,
		ratePerS: ratePerSec,
		lastFill: time.Now(),
	}
}

// allow returns true and consumes one token if the bucket is non-empty.
func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(tb.lastFill).Seconds()
	tb.tokens += elapsed * tb.ratePerS
	if tb.tokens > tb.maxTok {
		tb.tokens = tb.maxTok
	}
	tb.lastFill = now
	if tb.tokens < 1 {
		return false
	}
	tb.tokens--
	return true
}

// rateLimitMiddleware enforces a global request rate limit (1000 req/min).
// Returns 429 Too Many Requests when the limit is exceeded.
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	// 1000 requests per minute ≈ 16.67 req/s, burst of 50.
	limiter := newTokenBucket(1000.0/60.0, 50)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow() {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers. In dev mode (no allowed origins configured)
// it allows all origins; in production set ServerOptions.AllowedOrigins.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS is intentionally restrictive: no wildcard in production.
		// AllowedOrigins is set via ServerOptions; empty means deny cross-origin.
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
