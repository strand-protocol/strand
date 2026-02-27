package tenant

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/strand-protocol/strand/platform/auth/pkg/identity"
)

// RLSMiddleware sets the PostgreSQL RLS tenant context on each request.
// Must run after authentication middleware has populated the context.
func RLSMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := identity.FromContext(r.Context())
			if !ok {
				// No auth context -- skip RLS setup (RequireAuth will catch this)
				next.ServeHTTP(w, r)
				return
			}

			// Set the tenant context on the DB connection for this request.
			// Note: In production, use a per-request connection or transaction.
			conn, err := db.Conn(r.Context())
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"database connection: %s"}`, err), http.StatusInternalServerError)
				return
			}
			defer conn.Close()

			_, err = conn.ExecContext(r.Context(), "SET LOCAL app.current_tenant_id = $1", info.TenantID)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"set tenant context: %s"}`, err), http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
