package jwt

import (
	"encoding/json"
	"errors"
	"ride-hail/internal/domain/user"
	"strings"
)

var (
	ErrBadAuthMsg   = errors.New("invalid auth message")
	ErrBadTokenWrap = errors.New("token must be 'Bearer <token>'")
)

// ClientAuthMessage is what clients send first over WS:
// { "type":"auth", "token":"Bearer <jwt>" }
type ClientAuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

type Result struct {
	Claims *Claims
	Raw    string
}

// ValidateWSAuth parses the first auth frame, validates the JWT, and enforces RBAC. Used in WebSocket auth.
func ValidateWSAuth(frame []byte, mgr *Manager, allowedRoles ...user.Role) (*Result, error) {
	// parse auth message
	var msg ClientAuthMessage
	if err := json.Unmarshal(frame, &msg); err != nil {
		return nil, ErrBadAuthMsg
	}

	// validate message type and token format
	if strings.ToLower(strings.TrimSpace(msg.Type)) != "auth" {
		return nil, ErrBadAuthMsg
	}

	// expect "Bearer <token>" wrapping
	parts := strings.SplitN(msg.Token, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, ErrBadTokenWrap
	}

	// parse and validate token
	raw := strings.TrimSpace(parts[1])
	_, claims, err := mgr.ParseAndValidate(raw)
	if err != nil {
		return nil, err
	}

	// enforce role-based access control (RBAC)
	if err := RoleAllowed(claims, allowedRoles...); err != nil {
		return nil, err
	}

	return &Result{Claims: claims, Raw: raw}, nil
}
