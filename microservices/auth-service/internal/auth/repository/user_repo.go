package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/saurabhkr78/sudowallet/microservices/auth-service/internal/auth/model"
)

type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
}

type mysqlUserRepository struct {
	db *sql.DB
}

func NewMySQLUserRepository(db *sql.DB) UserRepository {
	return &mysqlUserRepository{db: db}
}

// users is owned by user-service; auth-service reads it (shared-database phase).
const userColumns = `id, full_name, email, password_hash, avatar_url, is_verified, created_at, updated_at`

func scanUser(row *sql.Row) (*model.User, error) {
	u := &model.User{}
	var avatarURL sql.NullString
	err := row.Scan(
		&u.ID, &u.FullName, &u.Email, &u.PasswordHash,
		&avatarURL, &u.IsVerified, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.AvatarURL = avatarURL.String
	return u, nil
}

func (r *mysqlUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE email = ? AND deleted_at IS NULL`
	u, err := scanUser(r.db.QueryRowContext(ctx, query, email))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return u, nil
}

func (r *mysqlUserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE id = ? AND deleted_at IS NULL`
	u, err := scanUser(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return u, nil
}
