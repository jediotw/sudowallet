package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/model"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
)

// TransactionRepository reads the transactions table (owned by
// transaction-service) for the daily report job.
type TransactionRepository interface {
	GetTransactionsByDate(ctx context.Context, start, end time.Time) ([]*model.Transaction, error)
}

type mySQLTransactionRepository struct {
	db *sql.DB
}

func NewMySQLTransactionRepository(db *sql.DB) TransactionRepository {
	return &mySQLTransactionRepository{db: db}
}

func (r *mySQLTransactionRepository) GetTransactionsByDate(ctx context.Context, start, end time.Time) ([]*model.Transaction, error) {
	query := `SELECT id, sender_wallet_id, receiver_wallet_id, amount, status, created_at
		FROM transactions
		WHERE created_at >= ? AND created_at < ?`
	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		logger.Error(ctx, "failed to query transactions by date", "error", err)
		return nil, err
	}
	defer rows.Close()

	txs := make([]*model.Transaction, 0)
	for rows.Next() {
		tx := &model.Transaction{}
		var senderWalletID sql.NullString
		if err := rows.Scan(
			&tx.ID, &senderWalletID, &tx.ReceiverWalletID, &tx.Amount,
			&tx.Status, &tx.CreatedAt,
		); err != nil {
			logger.Error(ctx, "failed to scan transaction", "error", err)
			return nil, err
		}
		if senderWalletID.Valid {
			tx.SenderWalletID = senderWalletID.String
		}
		txs = append(txs, tx)
	}
	if err := rows.Err(); err != nil {
		logger.Error(ctx, "error iterating transactions", "error", err)
		return nil, err
	}
	return txs, nil
}
