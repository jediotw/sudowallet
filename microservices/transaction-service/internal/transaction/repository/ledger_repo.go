package repository

import (
	"context"
	"database/sql"

	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/model"
	"github.com/shopspring/decimal"
)

// LedgerRepository writes double-entry bookkeeping rows and serves the
// payment-facing read queries (mutations history, reconciliation).
type LedgerRepository interface {
	CreateTx(ctx context.Context, entry *model.LedgerEntry, tx *sql.Tx) error
	GetEntriesByWalletID(ctx context.Context, walletID string) ([]*model.LedgerEntry, error)
	GetBalanceByWalletID(ctx context.Context, walletID string) (decimal.Decimal, error)
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

// GetEntriesByWalletID returns every ledger entry that changed a wallet. Used by
// payment-service for the mutations listing.
func (r *mysqlLedgerRepository) GetEntriesByWalletID(ctx context.Context, walletID string) ([]*model.LedgerEntry, error) {
	query := `SELECT id, wallet_id, transaction_id, amount, entry_type, created_at
		FROM ledger_entries WHERE wallet_id = ?`

	rows, err := r.db.QueryContext(ctx, query, walletID)
	if err != nil {
		logger.Error(ctx, "ledger repository entries lookup failed", "wallet_id", walletID, "error", err)
		return nil, err
	}
	defer rows.Close()

	entries := make([]*model.LedgerEntry, 0)
	for rows.Next() {
		entry := &model.LedgerEntry{}
		if err := rows.Scan(
			&entry.ID, &entry.WalletID, &entry.TransactionID, &entry.Amount,
			&entry.EntryType, &entry.CreatedAt,
		); err != nil {
			logger.Error(ctx, "ledger repository entries scan failed", "error", err)
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		logger.Error(ctx, "ledger repository entries iteration failed", "error", err)
		return nil, err
	}
	return entries, nil
}

// GetBalanceByWalletID recomputes a wallet's balance from its ledger entries:
// credits increase it, debits decrease it. Used for reconciliation.
func (r *mysqlLedgerRepository) GetBalanceByWalletID(ctx context.Context, walletID string) (decimal.Decimal, error) {
	query := `
		SELECT COALESCE(
			SUM(
				CASE
					WHEN LOWER(entry_type) = 'credit' THEN amount
					WHEN LOWER(entry_type) = 'debit' THEN -amount
					ELSE 0
				END
			),
			0
		)
		FROM ledger_entries
		WHERE wallet_id = ?`

	var balance decimal.Decimal
	if err := r.db.QueryRowContext(ctx, query, walletID).Scan(&balance); err != nil {
		logger.Error(ctx, "ledger repository balance lookup failed", "wallet_id", walletID, "error", err)
		return decimal.Zero, err
	}
	return balance, nil
}
