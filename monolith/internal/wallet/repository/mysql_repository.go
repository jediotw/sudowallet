package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	"github.com/saurabhkr78/sudowallet/monolith/internal/wallet/model"
	"github.com/shopspring/decimal"
)

type mySqlWalletRepository struct {
	db *sql.DB
}

func NewMySQLWalletRepository(db *sql.DB) WalletRepository {
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
	logger.Info(ctx, "wallet repository lookup started", "user_id", userID)
	//find the wallet by user id
	query := "SELECT id, user_id, balance, currency, created_at, updated_at, status, version FROM wallets WHERE user_id = ?"
	//create a new wallet empty model to hold the result
	w := &model.Wallet{}
	//execute the query using the db object and scan the result into the wallet model (SELECT single row → tx.QueryRowContext())
	//scan all required colummn of this row and
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&w.ID, &w.UserID, &w.Balance, &w.Currency, &w.CreatedAt, &w.UpdatedAt, &w.Status, &w.Version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn(ctx, "wallet repository lookup found no wallet", "user_id", userID)
			return nil, errors.New("wallet not found")
		}
		logger.Error(ctx, "wallet repository lookup failed", "user_id", userID, "error", err)
		return nil, err
	}
	logger.Info(ctx, "wallet repository lookup completed", "user_id", userID, "wallet_id", w.ID, "version", w.Version)
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
	logger.Info(ctx, "wallet repository balance update started", "wallet_id", walletID, "amount", amount, "expected_version", expectedVersion)

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
		logger.Error(ctx, "wallet repository balance update failed", "wallet_id", walletID, "error", err)
		return err
	}

	// Check how many wallet rows were actually updated.
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		logger.Error(ctx, "wallet repository rows affected failed", "wallet_id", walletID, "error", err)
		return err
	}
	logger.Info(ctx, "wallet repository balance update rows affected", "wallet_id", walletID, "rows_affected", rowsAffected, "expected_version", expectedVersion)

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
			logger.Error(ctx, "wallet repository wallet existence check failed", "wallet_id", walletID, "error", err)
			return err
		}

		// Wallet does not exist.
		// Return a sentinel error that can be checked by the caller.
		//Note:this is repository layer so here we dont need http status code we can return a custom error and the service layer will handle it and return the appropriate http status code to the handler layer
		if !exists {
			logger.Warn(ctx, "wallet repository update rejected: wallet not found", "wallet_id", walletID)
			return customErr.ErrWalletNotFound
		}

		// Wallet exists, but the version no longer matches.
		// Another request modified the wallet first.
		logger.Warn(ctx, "wallet repository update rejected: concurrent version change", "wallet_id", walletID, "expected_version", expectedVersion)
		return customErr.ErrConcurrentUpdate
	}

	logger.Info(ctx, "wallet repository balance update completed", "wallet_id", walletID)
	// Wallet was successfully updated.
	return nil
}
