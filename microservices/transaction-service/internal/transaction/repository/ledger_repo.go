package repository

import (
	"context"
	"database/sql"

	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/model"
)

// LedgerRepository writes double-entry bookkeeping rows. Read queries
// (mutations history, reconciliation) belong to payment-service.
type LedgerRepository interface {
	CreateTx(ctx context.Context, entry *model.LedgerEntry, tx *sql.Tx) error
}

type mysqlLedgerRepository struct {
	db *sql.DB
}

func NewMySQLLedgerRepository(db *sql.DB) LedgerRepository {
	return &mysqlLedgerRepository{db: db}
}

func (r *mysqlLedgerRepository) CreateTx(ctx context.Context, entry *model.LedgerEntry, tx *sql.Tx) error {
	query := `INSERT INTO ledger_entries (id, wallet_id, transaction_id, amount, entry_type, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := tx.ExecContext(
		ctx,
		query,
		entry.ID, entry.WalletID, entry.TransactionID, entry.Amount, entry.EntryType, entry.CreatedAt,
	)
	if err != nil {
		logger.Error(ctx, "ledger repository create failed", "ledger_entry_id", entry.ID, "error", err)
		return err
	}
	return nil
}
