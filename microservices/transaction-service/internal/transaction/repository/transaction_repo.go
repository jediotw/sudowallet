package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/saurabhkr78/sudowallet/microservices/shared/pagination"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/model"
)

// TransactionRepository defines the interface for transaction operations.
type TransactionRepository interface {
	CreateTx(ctx context.Context, t *model.Transaction, tx *sql.Tx) error
	// GetByIdempotencyKey returns nil, nil when no row matches the key.
	GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*model.Transaction, error)
	GetHistory(ctx context.Context, walletID string, params pagination.Params) ([]model.Transaction, int64, error)
	GetTransactionsByDate(ctx context.Context, start, end time.Time) ([]model.Transaction, error)
}

type mysqlTransactionRepository struct {
	db *sql.DB
}

func NewMySQLTransactionRepository(db *sql.DB) TransactionRepository {
	return &mysqlTransactionRepository{db: db}
}

func (r *mysqlTransactionRepository) CreateTx(ctx context.Context, t *model.Transaction, tx *sql.Tx) error {
	query := `INSERT INTO transactions (id, sender_wallet_id, receiver_wallet_id, amount, description, idempotency_key, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := tx.ExecContext(
		ctx,
		query,
		t.ID, t.SenderWalletID, t.ReceiverWalletID, t.Amount, t.Description, t.IdempotencyKey, t.Status, t.CreatedAt,
	)
	return err
}

func (r *mysqlTransactionRepository) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*model.Transaction, error) {
	query := `SELECT id, sender_wallet_id, receiver_wallet_id, amount, description, idempotency_key, status, created_at
			FROM transactions
			WHERE idempotency_key = ?`

	t := &model.Transaction{}
	var senderWalletID sql.NullString

	err := r.db.QueryRowContext(ctx, query, idempotencyKey).Scan(
		&t.ID,
		&senderWalletID,
		&t.ReceiverWalletID,
		&t.Amount,
		&t.Description,
		&t.IdempotencyKey,
		&t.Status,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if senderWalletID.Valid {
		t.SenderWalletID = &senderWalletID.String
	}

	return t, nil
}

func (r *mysqlTransactionRepository) GetHistory(ctx context.Context, walletID string, params pagination.Params) ([]model.Transaction, int64, error) {
	countQuery := `SELECT COUNT(*) FROM transactions WHERE (sender_wallet_id = ? OR receiver_wallet_id = ?)`
	var total int64
	var err error

	if params.Status != "" {
		countQuery += " AND status = ?"
		err = r.db.QueryRowContext(ctx, countQuery, walletID, walletID, params.Status).Scan(&total)
	} else {
		err = r.db.QueryRowContext(ctx, countQuery, walletID, walletID).Scan(&total)
	}
	if err != nil {
		return nil, 0, err
	}

	// Whitelist sort/order columns to prevent SQL injection.
	sortColumn := "created_at"
	if params.Sort == "amount" {
		sortColumn = "amount"
	}
	sortOrder := "DESC"
	if params.Order == "asc" {
		sortOrder = "ASC"
	}

	query := `SELECT id, sender_wallet_id, receiver_wallet_id, amount, description, idempotency_key, status, created_at
			FROM transactions WHERE (sender_wallet_id = ? OR receiver_wallet_id = ?)`

	var rows *sql.Rows
	if params.Status != "" {
		query += " AND status = ? ORDER BY " + sortColumn + " " + sortOrder + " LIMIT ? OFFSET ?"
		rows, err = r.db.QueryContext(ctx, query, walletID, walletID, params.Status, params.Limit, params.Offset())
	} else {
		query += " ORDER BY " + sortColumn + " " + sortOrder + " LIMIT ? OFFSET ?"
		rows, err = r.db.QueryContext(ctx, query, walletID, walletID, params.Limit, params.Offset())
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var txs []model.Transaction
	for rows.Next() {
		var t model.Transaction
		var sender sql.NullString
		if err := rows.Scan(
			&t.ID,
			&sender,
			&t.ReceiverWalletID,
			&t.Amount,
			&t.Description,
			&t.IdempotencyKey,
			&t.Status,
			&t.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if sender.Valid {
			t.SenderWalletID = &sender.String
		}
		txs = append(txs, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return txs, total, nil
}

// GetTransactionsByDate returns transactions created in [start, end). Used by
// payment-service for the daily report.
func (r *mysqlTransactionRepository) GetTransactionsByDate(ctx context.Context, start, end time.Time) ([]model.Transaction, error) {
	query := `SELECT id, sender_wallet_id, receiver_wallet_id, amount, description, idempotency_key, status, created_at
		FROM transactions
		WHERE created_at >= ? AND created_at < ?`

	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	txs := make([]model.Transaction, 0)
	for rows.Next() {
		var t model.Transaction
		var sender sql.NullString
		if err := rows.Scan(
			&t.ID,
			&sender,
			&t.ReceiverWalletID,
			&t.Amount,
			&t.Description,
			&t.IdempotencyKey,
			&t.Status,
			&t.CreatedAt,
		); err != nil {
			return nil, err
		}
		if sender.Valid {
			t.SenderWalletID = &sender.String
		}
		txs = append(txs, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return txs, nil
}
