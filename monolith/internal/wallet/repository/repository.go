package repository

import (
	"context"
	"database/sql"
	"github.com/saurabhkr78/sudowallet/monolith/internal/wallet/model"
	"github.com/shopspring/decimal"
)

type WalletRepository interface {
	CreateTx(ctx context.Context, wallet *model.Wallet, tx *sql.Tx) error
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, error)
	UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, amount decimal.Decimal, currentVersion int64) error
}
