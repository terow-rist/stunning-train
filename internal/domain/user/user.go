package user

import (
	"errors"
	"maps"
	"net/mail"
	"strings"
	"time"
)

// Attrs mirrors the JSONB 'attrs' column for extensible per-user attributes.
type Attrs map[string]any

// User is the domain entity corresponding to the `users` table.
type User struct {
	ID           string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Email        string
	Role         Role
	Status       Status
	PasswordHash string
	Attrs        Attrs
}

var (
	ErrInvalidEmail      = errors.New("invalid email address")
	ErrEmptyPasswordHash = errors.New("password hash cannot be empty")
	ErrBadTimestamps     = errors.New("updated_at cannot be before created_at")
)

// New constructs a new User entity. Caller provides ID (UUID as string) and already-hashed password.
func NewUser(email string, role Role, passwordHash string, attrs Attrs) (*User, error) {
	now := time.Now().UTC()
	user := &User{
		CreatedAt:    now,
		UpdatedAt:    now,
		Email:        strings.TrimSpace(email),
		Role:         role,
		Status:       StatusActive,
		PasswordHash: strings.TrimSpace(passwordHash),
		Attrs:        cloneAttrs(attrs),
	}
	if err := user.Validate(); err != nil {
		return nil, err
	}

	return user, nil
}

// Validate checks invariants of the User entity.
func (user *User) Validate() error {
	if _, err := mail.ParseAddress(user.Email); err != nil {
		return ErrInvalidEmail
	}
	if !user.Role.Valid() {
		return ErrInvalidRole
	}
	if !user.Status.Valid() {
		return ErrInvalidStatus
	}
	if user.PasswordHash == "" {
		return ErrEmptyPasswordHash
	}
	if !user.CreatedAt.IsZero() && !user.UpdatedAt.IsZero() && user.UpdatedAt.Before(user.CreatedAt) {
		return ErrBadTimestamps
	}
	return nil
}

// ----- Setters and helpers -----

// SetStatus transitions user status (e.g., to INACTIVE or BANNED). Updates UpdatedAt timestamp.
func (user *User) SetStatus(status Status) error {
	if !status.Valid() {
		return ErrInvalidStatus
	}
	user.Status = status
	user.touch()
	return nil
}

// SetRole changes user role (e.g., promote PASSENGER -> DRIVER). Updates UpdatedAt timestamp.
func (user *User) SetRole(role Role) error {
	if !role.Valid() {
		return ErrInvalidRole
	}
	user.Role = role
	user.touch()
	return nil
}

// UpdateEmail validates and updates email. Updates UpdatedAt timestamp.
func (user *User) UpdateEmail(email string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrInvalidEmail
	}
	user.Email = email
	user.touch()
	return nil
}

// cloneAttrs creates a shallow copy to keep domain invariants safe.
func cloneAttrs(a Attrs) Attrs {
	if a == nil {
		return make(Attrs)
	}
	cp := make(Attrs, len(a))
	maps.Copy(cp, a)
	return cp
}

// touch sets UpdatedAt to now (UTC).
func (user *User) touch() {
	user.UpdatedAt = time.Now().UTC()
}

// Convenience helpers.
func (user *User) IsActive() bool    { return user.Status.IsActive() }
func (user *User) IsDriver() bool    { return user.Role.IsDriver() }
func (user *User) IsPassenger() bool { return user.Role.IsPassenger() }
func (user *User) IsAdmin() bool     { return user.Role.IsAdmin() }
