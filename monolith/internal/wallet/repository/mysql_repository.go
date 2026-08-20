package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/wallet/model"
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
	w:= &model.Wallet{}
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
