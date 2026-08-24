package repository

import (
	"context"
	"database/sql"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/model"
)

type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	GetById(ctx context.Context, id string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, u *model.User) error
	CreateTx(ctx context.Context, u *model.User, tx *sql.Tx) error
	SoftDelete(ctx context.Context, id string) error
}
