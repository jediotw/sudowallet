package service

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/google/uuid"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	ledgerModel "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/model"
	ledgerRepo "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/repository"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	dto "github.com/saurabhkr78/sudowallet/monolith/internal/transaction/dto"
	txmodel "github.com/saurabhkr78/sudowallet/monolith/internal/transaction/model"
	transactionRepo "github.com/saurabhkr78/sudowallet/monolith/internal/transaction/repository"
	userRepo "github.com/saurabhkr78/sudowallet/monolith/internal/user/repository"
	walletRepo "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/repository"
)

type TransactionService interface {
	// CreateTransaction creates a new db transaction and updates the wallet balance atomically.
	Transfer(ctx context.Context, senderUserID string, req dto.TransferRequest) (*txmodel.Transaction, error)
}

// this service layer will be intracting with ledger table,wallet table and transaction table to perform the transaction operation also user table to get the user id and wallet id for the given user id
type transactionService struct {
	//dependencies of the service layer
	//we need to inject the repository layer into the service layer to perform the transaction operation
	//so we need to inject the transaction repository, wallet repository and ledger repository into the service layer
	txRepo     transactionRepo.TransactionRepository
	wallRepo   walletRepo.WalletRepository
	ledgerRepo ledgerRepo.LedgerRepository
	userRepo   userRepo.UserRepository
	db         *sql.DB
}

func NewTransactionService(txRepo transactionRepo.TransactionRepository, wRepo walletRepo.WalletRepository, lRepo ledgerRepo.LedgerRepository, uRepo userRepo.UserRepository, db *sql.DB) TransactionService {
	return &transactionService{
		txRepo:     txRepo,
		wallRepo:   wRepo,
		ledgerRepo: lRepo,
		userRepo:   uRepo,
		db:         db,
	}
}

func (s *transactionService) Transfer(
	ctx context.Context,
	senderUserID string,
	req dto.TransferRequest,
) (*txmodel.Transaction, error) {

	logger.Info(ctx,
		"transfer started",
		"sender_user_id", senderUserID,
		"receiver_email", req.ReceiverEmail,
		"amount", req.Amount,
		"idempotency_key", req.IdempotencyKey,
	)

	// Check if this transaction is already processed by checking
	// the idempotency key in the transaction table.
	existing, err := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		logger.Error(ctx,
			"idempotency lookup failed",
			"error", err,
			"idempotency_key", req.IdempotencyKey,
		)
		return nil, err
	}

	if existing != nil {
		logger.Info(ctx,
			"existing transaction found",
			"transaction_id", existing.ID,
			"idempotency_key", req.IdempotencyKey,
		)

		// Transaction was already processed.
		return existing, nil
	}

	logger.Info(ctx, "idempotency check passed", "idempotency_key", req.IdempotencyKey)

	// Find receiver by email.
	receiver, err := s.userRepo.GetByEmail(ctx, req.ReceiverEmail)
	if err != nil {
		logger.Error(ctx,
			"receiver lookup failed",
			"receiver_email", req.ReceiverEmail,
			"error", err,
		)

		return nil, customErr.NewAppError(
			http.StatusNotFound,
			"RECEIVER_NOT_FOUND",
			"Receiver not found",
		)
	}

	logger.Info(ctx,
		"receiver found",
		"receiver_id", receiver.ID,
	)

	// Start database transaction.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error(ctx,
			"failed to begin database transaction",
			"error", err,
		)

		return nil, customErr.ErrInternalServer
	}

	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logger.Error(ctx, "transfer database transaction rollback failed", "error", rollbackErr)
		}
	}()

	logger.Info(ctx, "database transaction started")

	// Find sender wallet.
	senderWallet, err := s.wallRepo.GetByUserID(ctx, senderUserID)
	if err != nil {
		logger.Error(ctx,
			"sender wallet lookup failed",
			"user_id", senderUserID,
			"error", err,
		)

		return nil, customErr.NewAppError(
			http.StatusNotFound,
			"SENDER_WALLET_NOT_FOUND",
			"Sender wallet not found",
		)
	}

	logger.Info(ctx,
		"sender wallet found",
		"wallet_id", senderWallet.ID,
		"balance", senderWallet.Balance,
		"version", senderWallet.Version,
	)

	// Find receiver wallet.
	receiverWallet, err := s.wallRepo.GetByUserID(ctx, receiver.ID)
	if err != nil {
		logger.Error(ctx,
			"receiver wallet lookup failed",
			"user_id", receiver.ID,
			"error", err,
		)

		return nil, customErr.NewAppError(
			http.StatusNotFound,
			"RECEIVER_WALLET_NOT_FOUND",
			"Receiver wallet not found",
		)
	}

	logger.Info(ctx,
		"receiver wallet found",
		"wallet_id", receiverWallet.ID,
		"balance", receiverWallet.Balance,
		"version", receiverWallet.Version,
	)

	// Sender and receiver cannot be the same wallet.
	if senderWallet.ID == receiverWallet.ID {
		logger.Info(ctx,
			"transfer to self rejected",
			"wallet_id", senderWallet.ID,
		)

		return nil, customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"Cannot transfer to self",
		)
	}

	// Check whether sender has sufficient balance.
	if senderWallet.Balance.LessThan(req.Amount) {
		logger.Info(ctx,
			"transfer rejected due to insufficient balance",
			"wallet_id", senderWallet.ID,
			"balance", senderWallet.Balance,
			"amount", req.Amount,
		)

		return nil, customErr.NewAppError(
			http.StatusBadRequest,
			"INSUFFICIENT_BALANCE",
			"Insufficient balance",
		)
	}

	logger.Info(ctx, "balance check passed",
		"sender_balance", senderWallet.Balance,
		"amount", req.Amount,
	)

	// ============================================================
	// OPTIMISTIC LOCK
	// ============================================================

	// Debit sender.
	// Negative amount reduces the wallet balance.
	logger.Info(ctx,
		"updating sender wallet",
		"wallet_id", senderWallet.ID,
		"amount", req.Amount.Neg(),
		"expected_version", senderWallet.Version,
	)

	err = s.wallRepo.UpdateBalanceTx(
		ctx,
		tx,
		senderWallet.ID,
		req.Amount.Neg(),
		int64(senderWallet.Version),
	)

	if err != nil {
		logger.Error(ctx,
			"sender wallet update failed",
			"wallet_id", senderWallet.ID,
			"error", err,
		)

		return nil, customErr.NewAppError(
			http.StatusInternalServerError,
			"SENDER_WALLET_UPDATE_FAILED",
			"Failed to update sender wallet balance",
		)
	}

	logger.Info(ctx,
		"sender wallet updated successfully",
		"wallet_id", senderWallet.ID,
	)

	// Credit receiver.
	// Positive amount increases the wallet balance.
	logger.Info(ctx,
		"updating receiver wallet",
		"wallet_id", receiverWallet.ID,
		"amount", req.Amount,
		"expected_version", receiverWallet.Version,
	)

	err = s.wallRepo.UpdateBalanceTx(
		ctx,
		tx,
		receiverWallet.ID,
		req.Amount,
		int64(receiverWallet.Version),
	)

	if err != nil {
		logger.Error(ctx,
			"receiver wallet update failed",
			"wallet_id", receiverWallet.ID,
			"error", err,
		)

		return nil, customErr.NewAppError(
			http.StatusInternalServerError,
			"RECEIVER_WALLET_UPDATE_FAILED",
			"Failed to update receiver wallet balance",
		)
	}

	logger.Info(ctx,
		"receiver wallet updated successfully",
		"wallet_id", receiverWallet.ID,
	)

	// Create transaction record.
	transaction := &txmodel.Transaction{
		ID:               uuid.New().String(),
		SenderWalletID:   &senderWallet.ID,
		ReceiverWalletID: receiverWallet.ID,
		Amount:           req.Amount,
		Description:      req.Description,
		IdempotencyKey:   req.IdempotencyKey,
		Status:           "success",
		CreatedAt:        time.Now(),
	}

	logger.Info(ctx,
		"creating transaction record",
		"transaction_id", transaction.ID,
	)

	err = s.txRepo.CreateTx(ctx, transaction, tx)
	if err != nil {
		logger.Error(ctx,
			"transaction creation failed",
			"transaction_id", transaction.ID,
			"error", err,
		)

		return nil, customErr.ErrInternalServer
	}

	logger.Info(ctx,
		"transaction record created successfully",
		"transaction_id", transaction.ID,
	)

	// ============================================================
	// SENDER LEDGER ENTRY
	// ============================================================

	senderDebitEntry := &ledgerModel.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      senderWallet.ID,
		TransactionID: transaction.ID,
		Amount:        req.Amount.Neg(),
		EntryType:     "debit",
		CreatedAt:     time.Now().UTC(),
	}

	logger.Info(ctx,
		"creating sender debit ledger entry",
		"ledger_entry_id", senderDebitEntry.ID,
		"wallet_id", senderWallet.ID,
		"transaction_id", transaction.ID,
		"amount", senderDebitEntry.Amount,
	)

	err = s.ledgerRepo.CreateTx(ctx, senderDebitEntry, tx)
	if err != nil {
		logger.Error(ctx,
			"sender debit ledger creation failed",
			"ledger_entry_id", senderDebitEntry.ID,
			"error", err,
		)

		return nil, customErr.ErrInternalServer
	}

	logger.Info(ctx,
		"sender debit ledger entry created successfully",
		"ledger_entry_id", senderDebitEntry.ID,
	)

	// ============================================================
	// RECEIVER LEDGER ENTRY
	// ============================================================

	receiverCreditEntry := &ledgerModel.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      receiverWallet.ID,
		TransactionID: transaction.ID,
		Amount:        req.Amount,
		EntryType:     "credit",
		CreatedAt:     time.Now().UTC(),
	}

	logger.Info(ctx,
		"creating receiver credit ledger entry",
		"ledger_entry_id", receiverCreditEntry.ID,
		"wallet_id", receiverWallet.ID,
		"transaction_id", transaction.ID,
		"amount", receiverCreditEntry.Amount,
	)

	err = s.ledgerRepo.CreateTx(ctx, receiverCreditEntry, tx)
	if err != nil {
		logger.Error(ctx,
			"receiver credit ledger creation failed",
			"ledger_entry_id", receiverCreditEntry.ID,
			"error", err,
		)

		return nil, customErr.ErrInternalServer
	}

	logger.Info(ctx,
		"receiver credit ledger entry created successfully",
		"ledger_entry_id", receiverCreditEntry.ID,
	)

	// ============================================================
	// COMMIT
	// ============================================================

	logger.Info(ctx,
		"committing transfer database transaction",
		"transaction_id", transaction.ID,
	)

	err = tx.Commit()
	if err != nil {
		logger.Error(ctx,
			"transfer transaction commit failed",
			"transaction_id", transaction.ID,
			"error", err,
		)

		return nil, customErr.ErrInternalServer
	}

	logger.Info(ctx,
		"transfer completed successfully",
		"transaction_id", transaction.ID,
		"sender_wallet_id", senderWallet.ID,
		"receiver_wallet_id", receiverWallet.ID,
		"amount", req.Amount,
	)

	return transaction, nil
}
