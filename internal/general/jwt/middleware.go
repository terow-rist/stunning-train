package jwt

import (
	"net/http"
	"ride-hail/internal/domain/user"
)

// AuthMiddleware validates tokens and injects claims into the request context. Used for HTTP routes.
func AuthMiddlewareFunc(mgr *Manager, allowedRoles ...user.Role) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// extract token from Authorization header
			raw, err := FromAuthorization(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			// parse and validate token
			_, claims, err := mgr.ParseAndValidate(raw)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			// enforce role-based access control (RBAC)
			if err := RoleAllowed(claims, allowedRoles...); err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}

			// inject claims into context and proceed to next handler
			ctx := InjectClaims(r.Context(), claims)
			next(w, r.WithContext(ctx))
		}
	}
}

// RequireClaims extracts JWT claims from the request context.
func RequireClaims(r *http.Request) *Claims {
	c, _ := FromContext(r.Context())
	return c
}
