package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/model"
	"github.com/shopspring/decimal"
)

type WalletRepository interface {
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, error)
	GetByID(ctx context.Context, id string) (*model.Wallet, error)
	Create(ctx context.Context, w *model.Wallet) error
	CreateTx(ctx context.Context, tx *sql.Tx, w *model.Wallet) error
	UpdateBalance(ctx context.Context, walletID string, amount decimal.Decimal, expectedVersion int64) error
	UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, amount decimal.Decimal, expectedVersion int64) error
}

type mysqlWalletRepository struct {
	db *sql.DB
}

func NewMySQLWalletRepository(db *sql.DB) WalletRepository {
	return &mysqlWalletRepository{db: db}
}

const walletColumns = `id, user_id, balance, currency, status, version, created_at, updated_at`

func scanWallet(row *sql.Row) (*model.Wallet, error) {
	w := &model.Wallet{}
	err := row.Scan(
		&w.ID, &w.UserID, &w.Balance, &w.Currency, &w.Status, &w.Version, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (r *mysqlWalletRepository) GetByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	logger.Info(ctx, "wallet repository lookup by user started", "user_id", userID)

	query := `SELECT ` + walletColumns + ` FROM wallets WHERE user_id = ? AND deleted_at IS NULL`
	w, err := scanWallet(r.db.QueryRowContext(ctx, query, userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn(ctx, "wallet repository lookup found no wallet", "user_id", userID)
			return nil, customErr.ErrWalletNotFound
		}
		logger.Error(ctx, "wallet repository lookup by user failed", "user_id", userID, "error", err)
		return nil, err
	}

	logger.Info(ctx, "wallet repository lookup by user completed", "user_id", userID, "wallet_id", w.ID, "version", w.Version)
	return w, nil
}

func (r *mysqlWalletRepository) GetByID(ctx context.Context, id string) (*model.Wallet, error) {
	logger.Info(ctx, "wallet repository lookup by id started", "wallet_id", id)

	query := `SELECT ` + walletColumns + ` FROM wallets WHERE id = ? AND deleted_at IS NULL`
	w, err := scanWallet(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn(ctx, "wallet repository lookup by id found no wallet", "wallet_id", id)
			return nil, customErr.ErrWalletNotFound
		}
		logger.Error(ctx, "wallet repository lookup by id failed", "wallet_id", id, "error", err)
		return nil, err
	}

	logger.Info(ctx, "wallet repository lookup by id completed", "wallet_id", w.ID, "version", w.Version)
	return w, nil
}

func (r *mysqlWalletRepository) Create(ctx context.Context, w *model.Wallet) error {
	return r.create(ctx, r.db, w)
}

func (r *mysqlWalletRepository) CreateTx(ctx context.Context, tx *sql.Tx, w *model.Wallet) error {
	return r.create(ctx, tx, w)
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (r *mysqlWalletRepository) create(ctx context.Context, execer sqlExecer, w *model.Wallet) error {
	query := `INSERT INTO wallets (id, user_id, balance, currency, status, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := execer.ExecContext(
		ctx,
		query,
		w.ID, w.UserID, w.Balance, w.Currency, w.Status, w.Version, w.CreatedAt, w.UpdatedAt,
	)
	return err
}

func (r *mysqlWalletRepository) UpdateBalance(ctx context.Context, walletID string, amount decimal.Decimal, expectedVersion int64) error {
	return r.updateBalance(ctx, r.db, walletID, amount, expectedVersion)
}

func (r *mysqlWalletRepository) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, amount decimal.Decimal, expectedVersion int64) error {
	return r.updateBalance(ctx, tx, walletID, amount, expectedVersion)
}

func (r *mysqlWalletRepository) updateBalance(ctx context.Context, execer sqlExecer, walletID string, amount decimal.Decimal, expectedVersion int64) error {
	logger.Info(ctx, "wallet repository balance update started", "wallet_id", walletID, "amount", amount, "expected_version", expectedVersion)

	// The version check gives optimistic concurrency control: the update only
	// succeeds if the wallet still has the version we originally read.
	query := `
		UPDATE wallets
		SET balance = balance + ?, version = version + 1, updated_at = ?
		WHERE id = ? AND version = ? AND deleted_at IS NULL
	`

	res, err := execer.ExecContext(ctx, query, amount, time.Now(), walletID, expectedVersion)
	if err != nil {
		logger.Error(ctx, "wallet repository balance update failed", "wallet_id", walletID, "error", err)
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		logger.Error(ctx, "wallet repository rows affected failed", "wallet_id", walletID, "error", err)
		return err
	}

	// Nothing updated: either the wallet does not exist, or its version changed
	// because another request updated it first.
	if rowsAffected == 0 {
		var exists bool
		existsQuery := `SELECT EXISTS(SELECT 1 FROM wallets WHERE id = ? AND deleted_at IS NULL)`
		if err := execer.QueryRowContext(ctx, existsQuery, walletID).Scan(&exists); err != nil {
			logger.Error(ctx, "wallet repository wallet existence check failed", "wallet_id", walletID, "error", err)
			return err
		}

		if !exists {
			logger.Warn(ctx, "wallet repository update rejected: wallet not found", "wallet_id", walletID)
			return customErr.ErrWalletNotFound
		}

		logger.Warn(ctx, "wallet repository update rejected: concurrent version change", "wallet_id", walletID, "expected_version", expectedVersion)
		return customErr.ErrConcurrentUpdate
	}

	logger.Info(ctx, "wallet repository balance update completed", "wallet_id", walletID)
	return nil
}
