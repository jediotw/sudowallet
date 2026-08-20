package repository

import (
	"context"
	"database/sql"
	"github.com/saurabhkr78/sudowallet/monolith/internal/wallet/model"
)

type WalletRepository interface {
	CreateTx(ctx context.Context, wallet *model.Wallet, tx *sql.Tx) error
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, error)
}
