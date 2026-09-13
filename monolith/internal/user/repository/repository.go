package repository

import (
	"context"
	"database/sql"

	commonDto "github.com/saurabhkr78/sudowallet/monolith/internal/common/dto"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/model"
)

type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	GetById(ctx context.Context, id string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, u *model.User) error
	CreateTx(ctx context.Context, u *model.User, tx *sql.Tx) error
	UpdateAvatar(ctx context.Context, id string, avatarURL string) error
	SoftDelete(ctx context.Context, id string) error
	UpdateVerificationStatus(ctx context.Context, id string, verified bool) error

	UpdateVerificationStatusTx(ctx context.Context, tx *sql.Tx, id string, verified bool) error
	UpdatePassword(ctx context.Context, userID string, passwordHash string) error
	GetAll(ctx context.Context, params commonDto.PaginationParams) ([]*model.User, int64, error)
	UpdateRole(ctx context.Context, id string, role string) error
}
