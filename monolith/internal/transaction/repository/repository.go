package repository

import (
	"context"
	"database/sql"
	pgnDto "github.com/saurabhkr78/sudowallet/monolith/internal/common/dto"
	"github.com/saurabhkr78/sudowallet/monolith/internal/transaction/model"
)

// TransactionRepository defines the interface for transaction operations.
type TransactionRepository interface {
	CreateTx(ctx context.Context, tx *model.Transaction, sqlTx *sql.Tx) error
	GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*model.Transaction, error)
	GetHistory(ctx context.Context, walletID string, params pgnDto.PaginationParams) ([]model.Transaction, int64, error)
}
