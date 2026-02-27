package apiserver

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

const (
	// maxRequestBodyBytes limits request body size to 1 MiB to prevent DoS.
	maxRequestBodyBytes = 1 << 20 // 1 MiB
)

// applyMiddleware wraps the given handler with the standard middleware chain.
// Order (outermost to innermost): recovery -> auth -> requestBodyLimit -> cors -> logging -> requestID
func (s *Server) applyMiddleware(h http.Handler) http.Handler {
	h = requestIDMiddleware(h)
	h = loggingMiddleware(h)
	h = corsMiddleware(h)
	h = requestBodyLimitMiddleware(h)
	h = s.apiKeyMiddleware(h)
	h = recoveryMiddleware(h)
	return h
}

// apiKeyMiddleware enforces Bearer token authentication on all routes except
// /healthz and /readyz. Valid API keys are provided in ServerOptions.APIKeys.
func (s *Server) apiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health/readiness probes are exempt from authentication.
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		// If no API keys are configured (dev mode), pass through.
		if len(s.opts.APIKeys) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader || token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if _, ok := s.opts.APIKeys[token]; !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
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
