package repository

import (
	"context"
	"database/sql"

	pgnDto "github.com/saurabhkr78/sudowallet/monolith/internal/common/dto"
	"github.com/saurabhkr78/sudowallet/monolith/internal/transaction/model"
)

//get the db into this repository layer and implement the methods of the interface

type MySQLTransactionRepository struct {
	db *sql.DB
}

// constructor function to create a new instance of the repository
func NewMySQLTransactionRepository(db *sql.DB) TransactionRepository {
	return &MySQLTransactionRepository{
		db: db,
	}
}

func (r *MySQLTransactionRepository) CreateTx(ctx context.Context, t *model.Transaction, tx *sql.Tx) error {
	query := `INSERT INTO transactions (id, sender_wallet_id, receiver_wallet_id, amount, description, idempotency_key, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	//execute the query using the transaction object
	//excute does not return any rows, it just executes the query and returns an error if any
	_, err := tx.ExecContext(ctx, query, t.ID, t.SenderWalletID, t.ReceiverWalletID, t.Amount, t.Description, t.IdempotencyKey, t.Status, t.CreatedAt)
	// If the database returns an unexpected error,
	// pass the original error to the service layer.
	if err != nil {
		return err
	}
	return nil
}

// I have an idempotency key → I want to find the transaction associated with that key → one key should correspond to at most one transaction → therefore use QueryRowContext() → scan the row into a Transaction model.
func (r *MySQLTransactionRepository) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*model.Transaction, error) {
	// Prepare the query to find the transaction associated with the given idempotency key.
	query := `SELECT id, sender_wallet_id, receiver_wallet_id, amount, description, idempotency_key, status, created_at 
			FROM transactions 
			WHERE idempotency_key = ?`

	// Create a new Transaction model to hold the result.
	t := &model.Transaction{}

	// sender_wallet_id can be NULL for transactions such as top-ups.
	// A SQL NULL value cannot be scanned directly into a Go string,
	// because a string cannot represent SQL NULL.
	//
	// Therefore, we use sql.NullString as the scan destination.
	// sql.NullString contains:
	//   - String: the actual string value
	//   - Valid: whether the database value is NOT NULL
	//
	// If the database value is NULL:
	//   Valid = false
	//
	// If the database value contains a string:
	//   Valid = true
	//   String = the actual wallet ID
	var senderWalletID sql.NullString
	// Execute the query using the db object and scan the result into the Transaction model.
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
	// If the sender_wallet_id is NOT NULL, assign its value to the Transaction model.
	if senderWalletID.Valid {
		t.SenderWalletID = &senderWalletID.String
	} else { //else assign nil to the SenderWalletID field of the Transaction model in string pointer format
		t.SenderWalletID = nil
	}

	if err != nil {
		if err == sql.ErrNoRows {
			// If no transaction is found, return nil and no error.
			return nil, nil
		}
		// If the database returns an unexpected error,
		// pass the original error to the service layer.
		return nil, err
	}

	return t, nil
}
func (r *MySQLTransactionRepository) GetHistory(ctx context.Context, walletID string, params pgnDto.PaginationParams) ([]model.Transaction, int64, error) {
	// counting total data for pagination meta
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

	// get the paginated data, use sort and order
	// important, use whitelist for sort and order to prevent sql injection
	sortColumn := "created_at"
	if params.Sort == "amount" {
		sortColumn = "amount"
	}

	sortOrder := "DESC"
	if params.Order == "asc" {
		sortOrder = "ASC"
	}

	query := `SELECT id, sender_wallet_id, receiver_wallet_id,
				amount, description, idempotency_key, status, created_at
			FROM transactions WHERE (sender_wallet_id = ? OR
			receiver_wallet_id = ?)`

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
		err := rows.Scan(
			&t.ID,
			&sender,
			&t.ReceiverWalletID,
			&t.Amount,
			&t.Description,
			&t.IdempotencyKey,
			&t.Status,
			&t.CreatedAt,
		)
		if err != nil {
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
