// strand-auth is a standalone authentication and authorization service for the Strand Protocol platform.
// It integrates Ory Kratos for user identity, custom API key management, and multi-tenant enforcement.
//
// In production, this service runs as a sidecar or is imported as a library by strand-cloud.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	var (
		addr       = flag.String("addr", ":8081", "Listen address")
		pgDSN      = flag.String("db", envOr("STRAND_DATABASE_URL", "postgres://strand:strand@localhost:5432/strand?sslmode=disable"), "PostgreSQL DSN")
		kratosURL  = flag.String("kratos", envOr("STRAND_KRATOS_URL", "http://localhost:4433"), "Ory Kratos public URL")
	)
	flag.Parse()

	log.Printf("strand-auth: starting on %s", *addr)
	log.Printf("strand-auth: postgres=%s kratos=%s", *pgDSN, *kratosURL)

	// TODO: Wire up HTTP server with session middleware, API key validation,
	// and proxy to strand-cloud. For now this is a placeholder binary that
	// validates the Go module compiles.
	fmt.Fprintf(os.Stdout, "strand-auth ready on %s\n", *addr)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
