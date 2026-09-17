package repository

import (
	"context"
	"database/sql"

	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/model"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
)

// WalletRepository is a read projection of the wallets table (owned by
// wallet-service). It exists only for reconciliation; it never writes.
type WalletRepository interface {
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, error)
	GetAll(ctx context.Context) ([]*model.Wallet, error)
}

type mySQLWalletRepository struct {
	db *sql.DB
}

func NewMySQLWalletRepository(db *sql.DB) WalletRepository {
	return &mySQLWalletRepository{db: db}
}

func (r *mySQLWalletRepository) GetByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	query := `SELECT id, user_id, balance FROM wallets WHERE user_id = ? AND deleted_at IS NULL`
	w := &model.Wallet{}
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&w.ID, &w.UserID, &w.Balance); err != nil {
		logger.Error(ctx, "failed to get wallet by user_id", "user_id", userID, "error", err)
		return nil, err
	}
	return w, nil
}

func (r *mySQLWalletRepository) GetAll(ctx context.Context) ([]*model.Wallet, error) {
	query := `SELECT id, user_id, balance FROM wallets WHERE deleted_at IS NULL`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		logger.Error(ctx, "failed to query wallets", "error", err)
		return nil, err
	}
	defer rows.Close()

	wallets := make([]*model.Wallet, 0)
	for rows.Next() {
		w := &model.Wallet{}
		if err := rows.Scan(&w.ID, &w.UserID, &w.Balance); err != nil {
			logger.Error(ctx, "failed to scan wallet", "error", err)
			return nil, err
		}
		wallets = append(wallets, w)
	}
	if err := rows.Err(); err != nil {
		logger.Error(ctx, "error iterating wallets", "error", err)
		return nil, err
	}
	return wallets, nil
}
