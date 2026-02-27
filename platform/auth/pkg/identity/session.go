package identity

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

const (
	ctxTenantID contextKey = "tenant_id"
	ctxUserID   contextKey = "user_id"
	ctxUserRole contextKey = "user_role"
	ctxEmail    contextKey = "email"
)

// AuthInfo holds the authenticated principal's context.
type AuthInfo struct {
	TenantID string
	UserID   string
	Role     string
	Email    string
}

// FromContext extracts auth info from the request context.
func FromContext(ctx context.Context) (*AuthInfo, bool) {
	tid, ok1 := ctx.Value(ctxTenantID).(string)
	uid, ok2 := ctx.Value(ctxUserID).(string)
	role, ok3 := ctx.Value(ctxUserRole).(string)
	email, _ := ctx.Value(ctxEmail).(string)
	if !ok1 || !ok2 || !ok3 {
		return nil, false
	}
	return &AuthInfo{TenantID: tid, UserID: uid, Role: role, Email: email}, true
}

// SetContext creates a new context with auth info attached.
func SetContext(ctx context.Context, info *AuthInfo) context.Context {
	ctx = context.WithValue(ctx, ctxTenantID, info.TenantID)
	ctx = context.WithValue(ctx, ctxUserID, info.UserID)
	ctx = context.WithValue(ctx, ctxUserRole, info.Role)
	ctx = context.WithValue(ctx, ctxEmail, info.Email)
	return ctx
}

// SessionMiddleware validates Kratos session cookies and populates the request context.
// Falls through to the next handler if no session cookie is present (allowing API key auth).
func SessionMiddleware(kratos *KratosClient, db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie := r.Header.Get("Cookie")
			if cookie == "" || !strings.Contains(cookie, "ory_kratos_session") {
				// No session cookie -- let API key middleware handle it
				next.ServeHTTP(w, r)
				return
			}

			session, err := kratos.ValidateSession(cookie, "")
			if err != nil {
				// Invalid session -- let API key middleware try
				next.ServeHTTP(w, r)
				return
			}

			// Look up user by Kratos identity ID
			var tenantID, userID, role, email string
			err = db.QueryRowContext(r.Context(),
				`SELECT u.tenant_id, u.id, u.role, u.email
				 FROM users u WHERE u.kratos_identity_id = $1 AND u.status = 'active'`,
				session.IdentityID,
			).Scan(&tenantID, &userID, &role, &email)
			if err != nil {
				// User not found in platform -- treat as unauthenticated
				next.ServeHTTP(w, r)
				return
			}

			info := &AuthInfo{
				TenantID: tenantID,
				UserID:   userID,
				Role:     role,
				Email:    email,
			}
			ctx := SetContext(r.Context(), info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth middleware rejects unauthenticated requests with 401.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := FromContext(r.Context()); !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole middleware rejects requests from users without the minimum role.
func RequireRole(minRole string) func(http.Handler) http.Handler {
	roleLevel := map[string]int{
		"viewer": 0, "operator": 1, "admin": 2, "owner": 3,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := FromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			if roleLevel[info.Role] < roleLevel[minRole] {
				http.Error(w, fmt.Sprintf(`{"error":"forbidden: requires %s role"}`, minRole), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
