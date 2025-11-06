package postgres

import (
	"context"
	"encoding/json"
	"ride-hail/internal/domain/user"
	"ride-hail/internal/ports"
)

// UserRepo persists users using pgx and plain SQL.
type UserRepo struct{}

// NewUserRepo constructs a new UserRepo.
func NewUserRepo() ports.UserRepository {
	return &UserRepo{}
}

// Create inserts a new user row.
func (repo *UserRepo) CreateUser(ctx context.Context, u *user.User) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	// if caller didn't pre-assign an ID, insert and get it back
	if u.ID == "" {
		var (
			createdAt      = u.CreatedAt
			updatedAt      = u.UpdatedAt
			attrsValue any = u.Attrs
		)

		if err := tx.QueryRow(ctx, `
			INSERT INTO users (email, role, status, password_hash, attrs)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at, updated_at
		`,
			u.Email,
			u.Role.String(),
			u.Status.String(),
			u.PasswordHash,
			attrsValue,
		).Scan(&u.ID, &createdAt, &updatedAt); err != nil {
			return err
		}

		u.CreatedAt = createdAt
		u.UpdatedAt = updatedAt
		return nil
	}

	// If caller provided an ID, insert explicitly
	err = tx.QueryRow(ctx, `
		INSERT INTO users (id, email, role, status, password_hash, attrs) 
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`,
		u.ID,
		u.Email,
		u.Role.String(),
		u.Status.String(),
		u.PasswordHash,
		u.Attrs, // pgx marshals to jsonb
	).Scan(&u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

// GetByID returns one user by id.
func (repo *UserRepo) GetByID(ctx context.Context, id string) (*user.User, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var (
		out            user.User
		roleText       string
		statusText     string
		attrsRaw       []byte
		passwordHashed string
	)

	err = tx.QueryRow(ctx, `
		SELECT
			id, created_at, updated_at,
			email, role, status, password_hash, attrs
		FROM users
		WHERE id = $1
	`, id).Scan(
		&out.ID, &out.CreatedAt, &out.UpdatedAt,
		&out.Email, &roleText, &statusText, &passwordHashed, &attrsRaw,
	)
	if err != nil {
		return nil, err
	}

	out.Role = user.Role(roleText)
	out.Status = user.Status(statusText)
	out.PasswordHash = passwordHashed

	// decode JSONB attrs (nullable but defaults to '{}' in schema)
	if len(attrsRaw) > 0 {
		var attrs user.Attrs
		if err := json.Unmarshal(attrsRaw, &attrs); err != nil {
			return nil, err
		}
		out.Attrs = attrs
	} else {
		out.Attrs = make(user.Attrs)
	}

	return &out, nil
}
