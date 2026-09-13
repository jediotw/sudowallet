package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/model"
)

type MySQLRefreshTokenRepository struct {
	db *sql.DB
}

func NewRefreshTokenRepository(db *sql.DB) RefreshTokenRepository {
	return &MySQLRefreshTokenRepository{db: db}
}

func (r *MySQLRefreshTokenRepository) Create(
	ctx context.Context,
	rt *model.RefreshToken,
) error {

	query := `
        INSERT INTO refresh_token (
            id,
            token,
            user_id,
            expires_at,
            revoked,
            revoked_at
        )
        VALUES (?, ?, ?, ?, ?, ?)
    `

	_, err := r.db.ExecContext(
		ctx,
		query,
		rt.ID,
		rt.Token,
		rt.UserID,
		rt.ExpiresAt,
		rt.Revoked,
		rt.RevokedAt,
	)

	return err
}

func (r *MySQLRefreshTokenRepository) GetByToken(
	ctx context.Context,
	token string,
) (*model.RefreshToken, error) {

	query := `
        SELECT
            id,
            user_id,
            token,
            expires_at,
            revoked,
            revoked_at,
            created_at,
            updated_at
        FROM refresh_token
        WHERE token = ?
    `

	rt := &model.RefreshToken{}

	err := r.db.QueryRowContext(
		ctx,
		query,
		token,
	).Scan(
		&rt.ID,
		&rt.UserID,
		&rt.Token,
		&rt.ExpiresAt,
		&rt.Revoked,
		&rt.RevokedAt,
		&rt.CreatedAt,
		&rt.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("token not found")
		}

		return nil, err
	}

	return rt, nil
}
func (r *MySQLRefreshTokenRepository) Revoke(
	ctx context.Context,
	token string,
) error {

	query := `
        UPDATE refresh_token
        SET
            revoked = TRUE,
            revoked_at = CURRENT_TIMESTAMP
        WHERE token = ?
    `

	_, err := r.db.ExecContext(
		ctx,
		query,
		token,
	)

	return err
}
func (r *MySQLRefreshTokenRepository) RevokeAllByUserID(
	ctx context.Context,
	userID string,
) error {

	query := `
        UPDATE refresh_token
        SET
            revoked = TRUE,
            revoked_at = CURRENT_TIMESTAMP
        WHERE user_id = ?
          AND revoked = FALSE
    `

	_, err := r.db.ExecContext(
		ctx,
		query,
		userID,
	)

	return err
}
