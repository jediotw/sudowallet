package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
)

var ErrUserNotFound = errors.New("user not found")

// NOTE: same shared-database rationale as wallet_repo.go. With a per-service DB,
// this becomes a call to user-service's internal lookup endpoint.
type UserRepository interface {
	GetIDByEmail(ctx context.Context, email string) (string, error)
}

type mysqlUserRepository struct {
	db *sql.DB
}

func NewMySQLUserRepository(db *sql.DB) UserRepository {
	return &mysqlUserRepository{db: db}
}

func (r *mysqlUserRepository) GetIDByEmail(ctx context.Context, email string) (string, error) {
	query := `SELECT id FROM users WHERE email = ? AND deleted_at IS NULL`

	var id string
	if err := r.db.QueryRowContext(ctx, query, email).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn(ctx, "user lookup by email found no user", "email", email)
			return "", ErrUserNotFound
		}
		logger.Error(ctx, "user lookup by email failed", "email", email, "error", err)
		return "", err
	}

	return id, nil
}
