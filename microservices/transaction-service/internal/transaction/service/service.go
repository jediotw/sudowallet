package service

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/shared/pagination"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/client"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/dto"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/model"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/repository"
)

type TransactionService interface {
	Transfer(ctx context.Context, senderUserID string, req dto.TransferRequest) (*model.Transaction, error)
	GetHistory(ctx context.Context, userID string, params pagination.Params) ([]model.Transaction, *pagination.Meta, error)
}

type transactionService struct {
	txRepo       repository.TransactionRepository
	ledgerRepo   repository.LedgerRepository
	userClient   client.UserClient
	walletClient client.WalletClient
	db           *sql.DB
}

func NewTransactionService(
	txRepo repository.TransactionRepository,
	ledgerRepo repository.LedgerRepository,
	userClient client.UserClient,
	walletClient client.WalletClient,
	db *sql.DB,
) TransactionService {
	return &transactionService{
		txRepo:       txRepo,
		ledgerRepo:   ledgerRepo,
		userClient:   userClient,
		walletClient: walletClient,
		db:           db,
	}
}

// Transfer orchestrates a transfer across service boundaries:
//
//  1. Resolve the receiver via user-service (user-service owns users).
//  2. Resolve both wallets via wallet-service (wallet-service owns wallets).
//  3. Debit the sender and credit the receiver through wallet-service's
//     internal adjust API, guarding each with optimistic concurrency.
//  4. Persist the transaction + double-entry ledger rows in transaction-service's
//     own database.
//
// Wallet mutations can no longer be atomic with the transaction insert (two
// different databases), so this is a saga: a failure after the debit is
// compensated by reversing the applied adjustments.
func (s *transactionService) Transfer(ctx context.Context, senderUserID string, req dto.TransferRequest) (*model.Transaction, error) {
	logger.Info(ctx, "transfer started", "sender_user_id", senderUserID, "receiver_email", req.ReceiverEmail, "amount", req.Amount, "idempotency_key", req.IdempotencyKey)

	// Idempotency: a key already on file returns the original result.
	existing, err := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		logger.Error(ctx, "idempotency lookup failed", "idempotency_key", req.IdempotencyKey, "error", err)
		return nil, customErr.ErrInternalServer
	}
	if existing != nil {
		logger.Info(ctx, "existing transfer returned", "transaction_id", existing.ID, "idempotency_key", req.IdempotencyKey)
		return existing, nil
	}

	// Resolve the receiver user by email (user-service owns the users table).
	receiverID, err := s.userClient.GetIDByEmail(ctx, req.ReceiverEmail)
	if err != nil {
		logger.Error(ctx, "receiver lookup failed", "receiver_email", req.ReceiverEmail, "error", err)
		if errors.Is(err, client.ErrUserNotFound) {
			return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_NOT_FOUND", "Receiver not found")
		}
		return nil, customErr.ErrInternalServer
	}

	// Resolve both wallets (wallet-service owns the wallets table).
	senderWallet, err := s.walletClient.ResolveByUserID(ctx, senderUserID)
	if err != nil {
		logger.Error(ctx, "sender wallet lookup failed", "user_id", senderUserID, "error", err)
		return nil, customErr.NewAppError(http.StatusNotFound, "SENDER_WALLET_NOT_FOUND", "Sender wallet not found")
	}
	receiverWallet, err := s.walletClient.ResolveByUserID(ctx, receiverID)
	if err != nil {
		logger.Error(ctx, "receiver wallet lookup failed", "user_id", receiverID, "error", err)
		return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_WALLET_NOT_FOUND", "Receiver wallet not found")
	}
	if senderWallet.ID == receiverWallet.ID {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INVALID_REQUEST", "cannot transfer to self")
	}
	if senderWallet.Balance.LessThan(req.Amount) {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INSUFFICIENT_BALANCE", "Insufficient balance")
	}

	// Debit sender (-amount), guarded by optimistic locking.
	if _, err := s.walletClient.AdjustBalance(ctx, senderWallet.ID, req.Amount.Neg(), senderWallet.Version); err != nil {
		logger.Error(ctx, "sender debit failed", "wallet_id", senderWallet.ID, "error", err)
		return nil, mapAdjustError(err, "SENDER_WALLET_UPDATE_FAILED", "Failed to update sender wallet balance")
	}

	// Credit receiver (+amount). On failure, compensate by refunding the sender.
	if _, err := s.walletClient.AdjustBalance(ctx, receiverWallet.ID, req.Amount, receiverWallet.Version); err != nil {
		logger.Error(ctx, "receiver credit failed, compensating", "wallet_id", receiverWallet.ID, "error", err)
		s.refundSender(ctx, senderWallet, req.Amount, "receiver credit failed")
		return nil, mapAdjustError(err, "RECEIVER_WALLET_UPDATE_FAILED", "Failed to update receiver wallet balance")
	}

	now := time.Now().UTC()
	transaction := &model.Transaction{
		ID:               uuid.New().String(),
		SenderWalletID:   &senderWallet.ID,
		ReceiverWalletID: receiverWallet.ID,
		Amount:           req.Amount,
		Description:      req.Description,
		IdempotencyKey:   req.IdempotencyKey,
		Status:           "success",
		CreatedAt:        now,
	}

	// Persist the transaction and ledger rows in transaction-service's DB. On
	// failure, reverse both wallet adjustments (compensation).
	if err := s.persistTransfer(ctx, transaction, senderWallet.ID, receiverWallet.ID, req.Amount); err != nil {
		logger.Error(ctx, "transfer persistence failed, compensating", "error", err)
		s.refundSender(ctx, senderWallet, req.Amount, "local persistence failed")
		s.reverseReceiver(ctx, receiverWallet, req.Amount, "local persistence failed")
		return nil, customErr.ErrInternalServer
	}

	logger.Info(ctx, "transfer committed", "transaction_id", transaction.ID, "amount", req.Amount)
	return transaction, nil
}

// persistTransfer writes the transaction and its two ledger entries in a single
// local database transaction.
func (s *transactionService) persistTransfer(
	ctx context.Context,
	transaction *model.Transaction,
	senderWalletID, receiverWalletID string,
	amount decimal.Decimal,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error(ctx, "database transaction begin failed", "error", err)
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logger.Error(ctx, "database transaction rollback failed", "error", rollbackErr)
		}
	}()

	if err := s.txRepo.CreateTx(ctx, transaction, tx); err != nil {
		logger.Error(ctx, "transaction record creation failed", "transaction_id", transaction.ID, "error", err)
		return err
	}

	// Double-entry rows: debit for the sender, credit for the receiver.
	senderEntry := &model.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      senderWalletID,
		TransactionID: transaction.ID,
		Amount:        amount.Neg(),
		EntryType:     "debit",
		CreatedAt:     transaction.CreatedAt,
	}
	receiverEntry := &model.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      receiverWalletID,
		TransactionID: transaction.ID,
		Amount:        amount,
		EntryType:     "credit",
		CreatedAt:     transaction.CreatedAt,
	}
	if err := s.ledgerRepo.CreateTx(ctx, senderEntry, tx); err != nil {
		logger.Error(ctx, "sender ledger entry creation failed", "error", err)
		return err
	}
	if err := s.ledgerRepo.CreateTx(ctx, receiverEntry, tx); err != nil {
		logger.Error(ctx, "receiver ledger entry creation failed", "error", err)
		return err
	}

	if err := tx.Commit(); err != nil {
		logger.Error(ctx, "transfer transaction commit failed", "transaction_id", transaction.ID, "error", err)
		return err
	}
	return nil
}

// refundSender credits the sender back when a transfer failed after the
// sender debit. It re-resolves the wallet to get the current version.
func (s *transactionService) refundSender(ctx context.Context, wallet *client.Wallet, amount decimal.Decimal, reason string) {
	w, err := s.walletClient.ResolveByUserID(ctx, wallet.UserID)
	if err != nil {
		logger.Error(ctx, "compensation: failed to resolve sender wallet", "wallet_id", wallet.ID, "user_id", wallet.UserID, "error", err)
		return
	}
	if _, err := s.walletClient.AdjustBalance(ctx, w.ID, amount, w.Version); err != nil {
		logger.Error(ctx, "compensation: failed to credit back sender", "wallet_id", w.ID, "reason", reason, "error", err)
		return
	}
	logger.Warn(ctx, "compensation: sender credited back", "wallet_id", w.ID, "reason", reason)
}

// reverseReceiver debits the receiver back when local persistence failed
// after the receiver credit.
func (s *transactionService) reverseReceiver(ctx context.Context, wallet *client.Wallet, amount decimal.Decimal, reason string) {
	w, err := s.walletClient.ResolveByUserID(ctx, wallet.UserID)
	if err != nil {
		logger.Error(ctx, "compensation: failed to resolve receiver wallet", "wallet_id", wallet.ID, "user_id", wallet.UserID, "error", err)
		return
	}
	if _, err := s.walletClient.AdjustBalance(ctx, w.ID, amount.Neg(), w.Version); err != nil {
		logger.Error(ctx, "compensation: failed to debit receiver", "wallet_id", w.ID, "reason", reason, "error", err)
		return
	}
	logger.Warn(ctx, "compensation: receiver debited back", "wallet_id", w.ID, "reason", reason)
}

func mapAdjustError(err error, defaultCode, defaultMsg string) error {
	var appErr *customErr.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return customErr.NewAppError(http.StatusInternalServerError, defaultCode, defaultMsg)
}

func (s *transactionService) GetHistory(ctx context.Context, userID string, params pagination.Params) ([]model.Transaction, *pagination.Meta, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit <= 0 {
		params.Limit = 10
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	wallet, err := s.walletClient.ResolveByUserID(ctx, userID)
	if err != nil {
		return nil, nil, customErr.NewAppError(http.StatusNotFound, "WALLET_NOT_FOUND", "Wallet not found")
	}

	txs, total, err := s.txRepo.GetHistory(ctx, wallet.ID, params)
	if err != nil {
		return nil, nil, customErr.ErrInternalServer
	}

	totalPages := int(total / int64(params.Limit))
	if total%int64(params.Limit) != 0 {
		totalPages++
	}

	meta := &pagination.Meta{
		Page:      params.Page,
		Limit:     params.Limit,
		Total:     total,
		TotalPage: totalPages,
	}

	return txs, meta, nil
}