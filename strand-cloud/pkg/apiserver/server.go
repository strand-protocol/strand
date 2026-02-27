// Package apiserver implements the Nexus Cloud REST API server.
package apiserver

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/ca"
	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/observability"
	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/store"
)

// Role represents the RBAC role assigned to an API key.
type Role int

const (
	// RoleViewer allows read-only operations (GET).
	RoleViewer Role = iota
	// RoleOperator allows read and write operations (GET, POST, PUT).
	RoleOperator
	// RoleAdmin allows all operations including DELETE and certificate revocation.
	RoleAdmin
)

// APIKeyInfo associates a Bearer token with its description and RBAC role.
type APIKeyInfo struct {
	Description string
	Role        Role
}

// ServerOptions holds optional configuration for the Server.
type ServerOptions struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	// APIKeys maps Bearer token → APIKeyInfo. When non-empty, all routes except
	// /healthz and /readyz require a valid Bearer token in the Authorization header.
	// Leave empty to disable authentication (dev/test mode only).
	APIKeys map[string]APIKeyInfo
	// AllowedOrigins lists the origins allowed for CORS. Reserved for future use.
	AllowedOrigins []string
}

// DefaultServerOptions returns sensible defaults.
func DefaultServerOptions() ServerOptions {
	return ServerOptions{
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// Server is the Nexus Cloud HTTP API server.
type Server struct {
	httpServer *http.Server
	store      store.Store
	ca         *ca.CA
	metrics    *observability.Metrics
	mux        *http.ServeMux
	opts       ServerOptions
}

// NewServer creates a Server wired to the given Store, CA, and options.
func NewServer(s store.Store, authority *ca.CA, opts ServerOptions) *Server {
	srv := &Server{
		store:   s,
		ca:      authority,
		metrics: observability.NewMetrics(),
		mux:     http.NewServeMux(),
		opts:    opts,
	}
	srv.registerRoutes()
	handler := srv.applyMiddleware(srv.mux)
	srv.httpServer = &http.Server{
		Handler:      handler,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
		IdleTimeout:  opts.IdleTimeout,
	}
	return srv
}

// ListenAndServe starts the HTTP server on the given address.
func (s *Server) ListenAndServe(addr string) error {
	s.httpServer.Addr = addr
	log.Printf("nexus-cloud API server listening on %s", addr)
	return s.httpServer.ListenAndServe()
}

// GracefulShutdown performs a graceful shutdown of the HTTP server.
func (s *Server) GracefulShutdown(ctx context.Context) error {
	log.Println("nexus-cloud API server shutting down")
	return s.httpServer.Shutdown(ctx)
}

// Handler returns the root http.Handler (useful for testing with httptest).
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}
