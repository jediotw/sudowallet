package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/model"
)

var ErrUserNotFound = errors.New("user not found")

// UserRepository writes and reads the users table (owned by this service).
type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, u *model.User) error
	UpdateAvatar(ctx context.Context, id string, avatarURL string) error
	UpdateVerificationStatus(ctx context.Context, id string, verified bool) error
	UpdatePassword(ctx context.Context, userID string, passwordHash string) error
	SoftDelete(ctx context.Context, id string) error
}

type mysqlUserRepository struct {
	db *sql.DB
}

func NewMySQLUserRepository(db *sql.DB) UserRepository {
	return &mysqlUserRepository{db: db}
}

const userColumns = `id, full_name, email, password_hash, avatar_url, is_verified, created_at, updated_at, deleted_at`

func (r *mysqlUserRepository) Create(ctx context.Context, u *model.User) error {
	query := `INSERT INTO users (id, full_name, email, password_hash, avatar_url, is_verified) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(
		ctx,
		query,
		u.ID, u.FullName, u.Email, u.PasswordHash, u.AvatarURL, u.IsVerified,
	)
	return err
}

func scanUser(row *sql.Row) (*model.User, error) {
	u := &model.User{}
	err := row.Scan(
		&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.AvatarURL, &u.IsVerified,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *mysqlUserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE id = ? AND deleted_at IS NULL`
	u, err := scanUser(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return u, nil
}

func (r *mysqlUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	logger.Info(ctx, "user repository email lookup started", "email", email)

	query := `SELECT ` + userColumns + ` FROM users WHERE email = ? AND deleted_at IS NULL`
	u, err := scanUser(r.db.QueryRowContext(ctx, query, email))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn(ctx, "user repository email lookup found no user", "email", email)
			return nil, ErrUserNotFound
		}
		logger.Error(ctx, "user repository email lookup failed", "email", email, "error", err)
		return nil, err
	}

	logger.Info(ctx, "user repository email lookup completed", "email", email, "user_id", u.ID)
	return u, nil
}

func (r *mysqlUserRepository) Update(ctx context.Context, u *model.User) error {
	query := `UPDATE users SET full_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, u.FullName, u.ID)
	return err
}

func (r *mysqlUserRepository) UpdateAvatar(ctx context.Context, id string, avatarURL string) error {
	query := `UPDATE users SET avatar_url = ? WHERE id = ? AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, avatarURL, id)
	return err
}

func (r *mysqlUserRepository) UpdateVerificationStatus(ctx context.Context, id string, verified bool) error {
	query := `UPDATE users SET is_verified = ? WHERE id = ? AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, verified, id)
	return err
}

func (r *mysqlUserRepository) UpdatePassword(ctx context.Context, userID string, passwordHash string) error {
	query := `UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, passwordHash, userID)
	return err
}

func (r *mysqlUserRepository) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE users SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
