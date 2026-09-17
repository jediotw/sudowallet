package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/model"
	"github.com/shopspring/decimal"
)

// NOTE: this is a temporary local copy of the wallet repository that exists only
// for the shared-database migration phase, so transfers can stay atomic in a
// single DB transaction exactly like the monolith. Once each service owns its
// own database, delete this and call wallet-service's /internal/wallets/adjust
// API instead (saga orchestration).

type WalletRepository interface {
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, error)
	UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, amount decimal.Decimal, expectedVersion int64) error
}

type mysqlWalletRepository struct {
	db *sql.DB
}

func NewMySQLWalletRepository(db *sql.DB) WalletRepository {
	return &mysqlWalletRepository{db: db}
}

func (r *mysqlWalletRepository) GetByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	query := `SELECT id, user_id, balance, currency, status, version, created_at, updated_at
			FROM wallets WHERE user_id = ? AND deleted_at IS NULL`

	w := &model.Wallet{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&w.ID, &w.UserID, &w.Balance, &w.Currency, &w.Status, &w.Version, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, customErr.ErrWalletNotFound
		}
		return nil, err
	}
	return w, nil
}

func (r *mysqlWalletRepository) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, amount decimal.Decimal, expectedVersion int64) error {
	logger.Info(ctx, "wallet repository balance update started", "wallet_id", walletID, "amount", amount, "expected_version", expectedVersion)

	query := `
		UPDATE wallets
		SET balance = balance + ?, version = version + 1, updated_at = ?
		WHERE id = ? AND version = ? AND deleted_at IS NULL
	`

	res, err := tx.ExecContext(ctx, query, amount, time.Now(), walletID, expectedVersion)
	if err != nil {
		logger.Error(ctx, "wallet repository balance update failed", "wallet_id", walletID, "error", err)
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		var exists bool
		existsQuery := `SELECT EXISTS(SELECT 1 FROM wallets WHERE id = ? AND deleted_at IS NULL)`
		if err := tx.QueryRowContext(ctx, existsQuery, walletID).Scan(&exists); err != nil {
			return err
		}

		if !exists {
			logger.Warn(ctx, "wallet repository update rejected: wallet not found", "wallet_id", walletID)
			return customErr.ErrWalletNotFound
		}

		logger.Warn(ctx, "wallet repository update rejected: concurrent version change", "wallet_id", walletID, "expected_version", expectedVersion)
		return customErr.ErrConcurrentUpdate
	}

	return nil
}
