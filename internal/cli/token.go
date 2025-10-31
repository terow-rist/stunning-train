package cli

import (
	"fmt"
	"ride-hail/internal/domain/user"
	"ride-hail/internal/general/jwt"
	"time"
)

// GenerateUserToken mints a short-lived JWT for a seeded user.
// It uses jwt.Manager and returns the raw token plus the claims.
//
// Typical use (dev-only):
//
//	token, _, err := cli.GenerateUserToken(secret, 2*time.Hour,
//	    "550e8400-e29b-41d4-a716-446655440001", "PASSENGER")
//
// Keep this package dev/internal only. Do not call it from production code paths.
func GenerateUserToken(secret string, userID string, roleStr string) (string, jwt.Claims, error) {
	// parse and validate the role
	role, err := user.ParseRole(roleStr)
	if err != nil {
		return "", jwt.Claims{}, fmt.Errorf("invalid role %q: %w", roleStr, err)
	}

	// set up a new JWT manager
	mgr := jwt.NewManager(secret, 2*time.Hour)

	// generate the JWT token given the user ID and its role
	token, claims, err := mgr.IssueUserToken(userID, role)
	if err != nil {
		return "", jwt.Claims{}, fmt.Errorf("issue token: %w", err)
	}

	return token, *claims, nil
}
