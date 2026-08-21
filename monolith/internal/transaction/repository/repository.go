package repository

import (
	"context"
	"database/sql"
	"github.com/saurabhkr78/sudowallet/monolith/internal/transaction/model"
)

// TransactionRepository defines the interface for transaction operations.
type TransactionRepository interface {
	CreateTx(ctx context.Context, tx *model.Transaction, sqlTx *sql.Tx) error
	GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*model.Transaction, error)
}
