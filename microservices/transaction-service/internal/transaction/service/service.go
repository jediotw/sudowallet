package service

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/shared/pagination"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/dto"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/model"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/repository"
)

type TransactionService interface {
	Transfer(ctx context.Context, senderUserID string, req dto.TransferRequest) (*model.Transaction, error)
	GetHistory(ctx context.Context, userID string, params pagination.Params) ([]model.Transaction, *pagination.Meta, error)
}

type transactionService struct {
	txRepo      repository.TransactionRepository
	ledgerRepo  repository.LedgerRepository
	walletRepo  repository.WalletRepository
	userRepo    repository.UserRepository
	db          *sql.DB
	redisClient *redis.Client
}

func NewTransactionService(
	txRepo repository.TransactionRepository,
	walletRepo repository.WalletRepository,
	ledgerRepo repository.LedgerRepository,
	userRepo repository.UserRepository,
	db *sql.DB,
	redisClient *redis.Client,
) TransactionService {
	return &transactionService{
		txRepo:      txRepo,
		walletRepo:  walletRepo,
		ledgerRepo:  ledgerRepo,
		userRepo:    userRepo,
		db:          db,
		redisClient: redisClient,
	}
}

// Transfer debits the sender's wallet, credits the receiver's wallet, and writes
// a transaction row plus two ledger entries — all atomically, with idempotency
// and optimistic concurrency control (version check) on the wallets.
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

	// Resolve the receiver user by email.
	receiverID, err := s.userRepo.GetIDByEmail(ctx, req.ReceiverEmail)
	if err != nil {
		logger.Error(ctx, "receiver lookup failed", "receiver_email", req.ReceiverEmail, "error", err)
		return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_NOT_FOUND", "Receiver not found")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error(ctx, "database transaction begin failed", "error", err)
		return nil, customErr.ErrInternalServer
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logger.Error(ctx, "database transaction rollback failed", "error", rollbackErr)
		}
	}()

	senderWallet, err := s.walletRepo.GetByUserID(ctx, senderUserID)
	if err != nil {
		logger.Error(ctx, "sender wallet lookup failed", "user_id", senderUserID, "error", err)
		return nil, customErr.NewAppError(http.StatusNotFound, "SENDER_WALLET_NOT_FOUND", "Sender wallet not found")
	}
	receiverWallet, err := s.walletRepo.GetByUserID(ctx, receiverID)
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

	// Debit sender (-amount), credit receiver (+amount), guarded by optimistic locking.
	if err := s.walletRepo.UpdateBalanceTx(ctx, tx, senderWallet.ID, req.Amount.Neg(), int64(senderWallet.Version)); err != nil {
		logger.Error(ctx, "sender wallet update failed", "wallet_id", senderWallet.ID, "error", err)
		return nil, customErr.NewAppError(http.StatusInternalServerError, "SENDER_WALLET_UPDATE_FAILED", "Failed to update sender wallet balance")
	}
	if err := s.walletRepo.UpdateBalanceTx(ctx, tx, receiverWallet.ID, req.Amount, int64(receiverWallet.Version)); err != nil {
		logger.Error(ctx, "receiver wallet update failed", "wallet_id", receiverWallet.ID, "error", err)
		return nil, customErr.NewAppError(http.StatusInternalServerError, "RECEIVER_WALLET_UPDATE_FAILED", "Failed to update receiver wallet balance")
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
	if err := s.txRepo.CreateTx(ctx, transaction, tx); err != nil {
		logger.Error(ctx, "transaction record creation failed", "transaction_id", transaction.ID, "error", err)
		return nil, customErr.ErrInternalServer
	}

	// Double-entry rows: debit for the sender, credit for the receiver.
	senderEntry := &model.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      senderWallet.ID,
		TransactionID: transaction.ID,
		Amount:        req.Amount.Neg(),
		EntryType:     "debit",
		CreatedAt:     now,
	}
	receiverEntry := &model.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      receiverWallet.ID,
		TransactionID: transaction.ID,
		Amount:        req.Amount,
		EntryType:     "credit",
		CreatedAt:     now,
	}
	if err := s.ledgerRepo.CreateTx(ctx, senderEntry, tx); err != nil {
		logger.Error(ctx, "sender ledger entry creation failed", "error", err)
		return nil, customErr.ErrInternalServer
	}
	if err := s.ledgerRepo.CreateTx(ctx, receiverEntry, tx); err != nil {
		logger.Error(ctx, "receiver ledger entry creation failed", "error", err)
		return nil, customErr.ErrInternalServer
	}

	if err := tx.Commit(); err != nil {
		logger.Error(ctx, "transfer transaction commit failed", "transaction_id", transaction.ID, "error", err)
		return nil, customErr.ErrInternalServer
	}
	logger.Info(ctx, "transfer committed", "transaction_id", transaction.ID, "amount", req.Amount)

	// The wallet-service cache is keyed by user ID; drop both copies so the next
	// reads are fresh. Do it in the background so the response never waits.
	go s.invalidateWalletCache(ctx, senderUserID, receiverID)

	return transaction, nil
}

func (s *transactionService) invalidateWalletCache(ctx context.Context, senderUserID, receiverUserID string) {
	bgCtx := context.Background()
	if err := s.redisClient.Del(bgCtx, "wallet:user:"+senderUserID, "wallet:user:"+receiverUserID).Err(); err != nil {
		logger.Error(bgCtx, "failed to delete wallet cache after transfer", "sender_user_id", senderUserID, "receiver_user_id", receiverUserID, "error", err)
	}
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

	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
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
