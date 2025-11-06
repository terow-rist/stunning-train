package jwt

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"ride-hail/internal/domain/user"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

var (
	ErrNoAuthHeader       = errors.New("authorization header missing")
	ErrBadAuthScheme      = errors.New("authorization must start with Bearer")
	ErrEmptyToken         = errors.New("bearer token missing")
	ErrInvalidSigningAlgo = errors.New("unexpected signing method")
	ErrRoleForbidden      = errors.New("role not allowed")
)

// Manager handles JWT creation and validation.
type Manager struct {
	secret    []byte
	accessTTL time.Duration
}

// NewManager creates a token manager.
func NewManager(secret string, accessTTL time.Duration) *Manager {
	s := strings.TrimSpace(secret)
	if s == "" {
		panic("jwt: empty secret key")
	}

	return &Manager{
		secret:    []byte(s),
		accessTTL: accessTTL,
	}
}

// IssueUserToken returns a signed access token for a user (passenger/driver/admin).
func (m *Manager) IssueUserToken(userID string, role user.Role) (string, *Claims, error) {
	// validate role
	if !role.Valid() {
		return "", nil, fmt.Errorf("invalid role: %s", role)
	}

	// create claims and sign token
	claims := NewUserClaims(userID, role, m.accessTTL)
	tkn := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	signed, err := tkn.SignedString(m.secret)

	return signed, claims, err
}

// FromAuthorization reads "Authorization: Bearer <token>".
// In your jwt package, update FromAuthorization function
func FromAuthorization(r *http.Request) (string, error) {
	// First check Authorization header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer "), nil
	}

	// Then check query parameter (for WebSocket connections)
	if authParam := r.URL.Query().Get("Authorization"); authParam != "" {
		if strings.HasPrefix(authParam, "Bearer ") {
			return strings.TrimPrefix(authParam, "Bearer "), nil
		}
		return authParam, nil // Some clients send just the token
	}

	return "", fmt.Errorf("missing or malformed Authorization")
}

// ParseAndValidate verifies signature and standard claims.
func (m *Manager) ParseAndValidate(tokenString string) (*jwtlib.Token, *Claims, error) {
	// create parser with expected signing method
	parser := jwtlib.NewParser(jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Alg()}))

	// validate claims and signature
	claims := &Claims{}
	token, err := parser.ParseWithClaims(tokenString, claims, func(t *jwtlib.Token) (any, error) {
		if t.Method != jwtlib.SigningMethodHS256 {
			return nil, ErrInvalidSigningAlgo
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, nil, err
	}

	// ensure token is valid
	if !token.Valid {
		return nil, nil, errors.New("invalid token")
	}

	return token, claims, nil
}

// RoleAllowed asserts the claims' role is one of the allowed.
func RoleAllowed(cl *Claims, allowed ...user.Role) error {
	if slices.Contains(allowed, cl.Role) {
		return nil
	}
	return ErrRoleForbidden
}

// Context wiring (used by middleware)
type ctxKey string

const claimsCtxKey ctxKey = "jwtClaims"

// InjectClaims adds JWT claims to the context.
func InjectClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsCtxKey, c)
}

// FromContext extracts JWT claims from the context.
func FromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsCtxKey).(*Claims)
	return c, ok
}
