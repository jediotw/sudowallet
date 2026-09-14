package repository

import (
	"context"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/model"
)

type RefreshTokenRepository interface {
	Create(ctx context.Context, rt *model.RefreshToken) error
	GetByToken(ctx context.Context, token string) (*model.RefreshToken, error)
	Revoke(ctx context.Context, token string) error
	RevokeAllByUserID(ctx context.Context, userID string) error
}
