package repository

import (
	"context"
	"database/sql"

	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/model"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/shopspring/decimal"
)

// LedgerRepository reads the ledger_entries table (owned by transaction-service)
// for the balance/mutation API and the reconciliation job.
type LedgerRepository interface {
	GetEntriesByWalletID(ctx context.Context, walletID string) ([]*model.LedgerEntry, error)
	GetBalanceByWalletID(ctx context.Context, walletID string) (decimal.Decimal, error)
}

type mySQLLedgerRepository struct {
	db *sql.DB
}

func NewMySQLLedgerRepository(db *sql.DB) LedgerRepository {
	return &mySQLLedgerRepository{db: db}
}

func (r *mySQLLedgerRepository) GetEntriesByWalletID(ctx context.Context, walletID string) ([]*model.LedgerEntry, error) {
	query := `SELECT id, wallet_id, transaction_id, amount, entry_type, created_at
		FROM ledger_entries WHERE wallet_id = ?`
	rows, err := r.db.QueryContext(ctx, query, walletID)
	if err != nil {
		logger.Error(ctx, "failed to query ledger entries", "wallet_id", walletID, "error", err)
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
			logger.Error(ctx, "failed to scan ledger entry", "error", err)
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		logger.Error(ctx, "error iterating ledger entries", "error", err)
		return nil, err
	}
	return entries, nil
}

// GetBalanceByWalletID recomputes a wallet's balance from its ledger entries:
// credits increase it, debits decrease it.
func (r *mySQLLedgerRepository) GetBalanceByWalletID(ctx context.Context, walletID string) (decimal.Decimal, error) {
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
		logger.Error(ctx, "failed to compute ledger balance", "wallet_id", walletID, "error", err)
		return decimal.Zero, err
	}
	return balance, nil
}
