package jwt

import (
	"ride-hail/internal/domain/user"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// Claims defines our canonical JWT claims payload.
type Claims struct {
	Role user.Role `json:"role"` // user role for RBAC (PASSENGER/DRIVER/ADMIN)
	jwtlib.RegisteredClaims
}

// ensure Claims implements jwtlib.Claims interface
var _ jwtlib.Claims = (*Claims)(nil)

// NewUserClaims constructs end-user claims (passenger/driver/admin).
func NewUserClaims(userID string, role user.Role, ttl time.Duration) *Claims {
	now := time.Now().UTC()
	return &Claims{
		Role: role,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwtlib.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwtlib.NewNumericDate(now),
		},
	}
}
