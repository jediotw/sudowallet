package repository

import (
	"context"
	"database/sql"
	"errors"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/wallet/model"
	"github.com/shopspring/decimal"
	"time"
)

type mySqlWalletRepository struct {
	db *sql.DB
}

func NewMySqlWalletRepository(db *sql.DB) WalletRepository {
	return &mySqlWalletRepository{
		db: db,
	}
}

func (r *mySqlWalletRepository) CreateTx(ctx context.Context, wallet *model.Wallet, tx *sql.Tx) error {
	//prepare the insert statement
	query := "INSERT INTO wallets (id, user_id, balance, currency, created_at, updated_at, status, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	//now we jave query and we can execute it using the transaction using ExecuteContext method of the transaction object which takes context, query and the parameters to be inserted into the query
	//this method can be used for UPDATE/INSERT/DELETE
	_, err := tx.ExecContext(ctx, query, wallet.ID, wallet.UserID, wallet.Balance, wallet.Currency, wallet.CreatedAt, wallet.UpdatedAt, wallet.Status, wallet.Version)
	if err != nil {
		return err
	}
	return nil
}
func (r *mySqlWalletRepository) GetByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	//find the wallet by user id
	query := "SELECT id, user_id, balance, currency, created_at, updated_at, status, version FROM wallets WHERE user_id = ?"
	//create a new wallet empty model to hold the result
	w := &model.Wallet{}
	//execute the query using the db object and scan the result into the wallet model (SELECT single row → tx.QueryRowContext())
	//scan all required colummn of this row and
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&w.ID, &w.UserID, &w.Balance, &w.Currency, &w.CreatedAt, &w.UpdatedAt, &w.Status, &w.Version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("wallet not found")
		}
		return nil, err
	}
	//else return the wallet model
	return w, nil

}

// lets make this function a genric function Then the caller decides whether it's a credit or debit.
func (r *mySqlWalletRepository) UpdateBalanceTx(
	ctx context.Context,
	tx *sql.Tx,
	walletID string,
	amount decimal.Decimal,
	expectedVersion int64,
) error {

	// Update the wallet balance and increment the version.
	// The version check provides optimistic concurrency control.
	// The update only succeeds if the wallet still has the
	// version that we originally read.
	query := `
        UPDATE wallets
        SET
            balance = balance + ?,
            version = version + 1,
            updated_at = ?
        WHERE id = ?
          AND version = ?
    `

	// Execute the update using the existing transaction.
	// A positive amount increases the balance, while a negative
	// amount decreases the balance.
	res, err := tx.ExecContext(
		ctx,
		query,
		amount,
		time.Now(),
		walletID,
		expectedVersion,
	)

	if err != nil {
		return err
	}

	// Check how many wallet rows were actually updated.
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	// No row was updated. This means either:
	// 1. The wallet does not exist.
	// 2. The wallet exists, but its version has changed because
	//    another request updated it first.
	if rowsAffected == 0 {

		// Check whether the wallet exists.
		var exists bool
		// query to find out if the wallet exists
		query := `SELECT EXISTS(SELECT 1 FROM wallets WHERE id = ?)`
		err := tx.QueryRowContext(ctx, query, walletID).Scan(&exists)

		if err != nil {
			return err
		}

		// Wallet does not exist.
		// Return a sentinel error that can be checked by the caller.
		//Note:this is repository layer so here we dont need http status code we can return a custom error and the service layer will handle it and return the appropriate http status code to the handler layer
		if !exists {
			return customErr.ErrWalletNotFound
		}

		// Wallet exists, but the version no longer matches.
		// Another request modified the wallet first.
		return customErr.ErrConcurrentUpdate
	}

	// Wallet was successfully updated.
	return nil
}
