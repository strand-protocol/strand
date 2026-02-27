package apiserver

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
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
// Order (outermost to innermost): recovery -> auth -> rbac -> rateLimiter -> requestBodyLimit -> cors -> securityHeaders -> logging -> requestID
func (s *Server) applyMiddleware(h http.Handler) http.Handler {
	h = requestIDMiddleware(h)
	h = loggingMiddleware(h)
	h = securityHeadersMiddleware(h)
	h = s.corsMiddleware(h)
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
		// Constant-time comparison: iterate all keys to avoid timing
		// side-channels that leak which tokens are valid.
		var matchedInfo APIKeyInfo
		var found bool
		for key, info := range s.opts.APIKeys {
			if subtle.ConstantTimeCompare([]byte(key), []byte(token)) == 1 {
				matchedInfo = info
				found = true
				// Do NOT break: must iterate all keys for constant-time behaviour.
			}
		}
		if !found {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), roleContextKey, matchedInfo.Role)
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

// remaining returns the current number of available tokens (for rate-limit headers).
func (tb *tokenBucket) remaining() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(tb.lastFill).Seconds()
	tokens := tb.tokens + elapsed*tb.ratePerS
	if tokens > tb.maxTok {
		tokens = tb.maxTok
	}
	return tokens
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
		w.Header().Set("X-RateLimit-Limit", "1000")
		remaining := int(limiter.remaining())
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		if !limiter.allow() {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware adds defensive HTTP headers to every response.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers. When AllowedOrigins is configured, only
// those origins are reflected; when empty (dev mode), all origins are allowed
// with a log warning on first request.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	var warnOnce sync.Once
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

		if origin != "" {
			if len(s.opts.AllowedOrigins) == 0 {
				// Dev mode: allow all origins but warn.
				warnOnce.Do(func() {
					log.Println("WARNING: AllowedOrigins is empty — allowing all CORS origins (dev mode)")
				})
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else {
				for _, allowed := range s.opts.AllowedOrigins {
					if allowed == origin {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Set("Access-Control-Allow-Credentials", "true")
						break
					}
				}
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
