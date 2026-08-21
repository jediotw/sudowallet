package repository

import (
	"context"
	"database/sql"
	"github.com/saurabhkr78/sudowallet/monolith/internal/transaction/model"
)

//get the db into this repository layer and implement the methods of the interface

type mySqlTransactionRepository struct {
	db *sql.DB
}

// constructor function to create a new instance of the repository
func NewMySqlTransactionRepository(db *sql.DB) TransactionRepository {
	return &mySqlTransactionRepository{
		db: db,
	}
}

func (r *mySqlTransactionRepository) CreateTx(ctx context.Context, t *model.Transaction, tx *sql.Tx) error {
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
func (r *mySqlTransactionRepository) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*model.Transaction, error) {
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
