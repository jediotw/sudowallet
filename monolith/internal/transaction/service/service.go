package service

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	pgnDto "github.com/saurabhkr78/sudowallet/monolith/internal/common/dto"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	ledgerModel "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/model"
	ledgerRepo "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/repository"
	dto "github.com/saurabhkr78/sudowallet/monolith/internal/transaction/dto"
	txmodel "github.com/saurabhkr78/sudowallet/monolith/internal/transaction/model"
	transactionRepo "github.com/saurabhkr78/sudowallet/monolith/internal/transaction/repository"
	userRepo "github.com/saurabhkr78/sudowallet/monolith/internal/user/repository"
	walletRepo "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/repository"
	"net/http"
	"time"
)

type TransactionService interface {
	// CreateTransaction creates a new db transaction and updates the wallet balance atomically.
	Transfer(ctx context.Context, senderUserID string, req dto.TransferRequest) (*txmodel.Transaction, error)
	GetHistory(ctx context.Context, userID string, params pgnDto.PaginationParams) ([]txmodel.Transaction, *pgnDto.PaginationMeta, error)
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

func (s *transactionService) Transfer(ctx context.Context, senderUserID string, req dto.TransferRequest) (*txmodel.Transaction, error) {
<<<<<<< Updated upstream
	// check if this transaction is already processed by checking the idempotency key in the transaction table
	existing, _ := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if existing != nil {
=======
	logger.Info(ctx, "transfer started", "sender_user_id", senderUserID, "receiver_email", req.ReceiverEmail, "amount", req.Amount, "idempotency_key", req.IdempotencyKey)
	// check if this transaction is already processed by checking the idempotency key in the transaction table
	existing, err := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		logger.Error(ctx, "idempotency lookup failed", "idempotency_key", req.IdempotencyKey, "error", err)
		return nil, customErr.ErrInternalServer
	}
	if existing != nil {
		logger.Info(ctx, "existing transfer returned", "transaction_id", existing.ID, "idempotency_key", req.IdempotencyKey)
>>>>>>> Stashed changes
		//if the transaction is already processed then return the existing transaction
		return existing, nil
	}
	// find receiver by email it returns the user model which contains the user id and other details
	receiver, err := s.userRepo.GetByEmail(ctx, req.ReceiverEmail)
	if err != nil {
<<<<<<< Updated upstream
=======
		logger.Error(ctx, "receiver lookup failed", "receiver_email", req.ReceiverEmail, "error", err)
>>>>>>> Stashed changes
		return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_NOT_FOUND", "Receiver not found")
	}
	// start db transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
<<<<<<< Updated upstream
		return nil, customErr.ErrInternalServer
	}
	defer tx.Rollback() // rollback the transaction if any error occurs
=======
		logger.Error(ctx, "database transaction begin failed", "error", err)
		return nil, customErr.ErrInternalServer
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logger.Error(ctx, "database transaction rollback failed", "error", rollbackErr)
		}
	}()
	logger.Info(ctx, "database transaction started")
>>>>>>> Stashed changes

	// look for sender and receivver wallet by their ids
	senderWalletId, err := s.wallRepo.GetByUserID(ctx, senderUserID)
	if err != nil {
<<<<<<< Updated upstream
=======
		logger.Error(ctx, "sender wallet lookup failed", "user_id", senderUserID, "error", err)
>>>>>>> Stashed changes
		return nil, customErr.NewAppError(http.StatusNotFound, "SENDER_WALLET_NOT_FOUND", "Sender wallet not found")
	}
	receiverWalletId, err := s.wallRepo.GetByUserID(ctx, receiver.ID)
	if err != nil {
<<<<<<< Updated upstream
=======
		logger.Error(ctx, "receiver wallet lookup failed", "user_id", receiver.ID, "error", err)
>>>>>>> Stashed changes
		return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_WALLET_NOT_FOUND", "Receiver wallet not found")
	}
	// sender and receiver cannot be same
	if senderWalletId.ID == receiverWalletId.ID {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INVALID_REQUEST", "cannot transfer to self")
	}
	// check if the sender has sufficient balance to transfer the amount
	if senderWalletId.Balance.LessThan(req.Amount) {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INSUFFICIENT_BALANCE", "Insufficient balance")
	}
	//new balance for sender and receiver after the transaction

	// the amount passed to it must represent the change in balance, not the new balance by subtracting or adding the amount to the current balance.

	//update both wallet balance in the wallet table using the UpdateBalanceTx method of the wallet repository
	// if amount is +ve then it is a credit transaction and if amount is -ve then it is a debit transaction
	//debit -ve amount
	//here we apply optimistic locking by passing the expected version of the wallet to the UpdateBalanceTx method of the wallet repository, so that if the wallet is updated by another transaction in between then this transaction will fail and return an error
<<<<<<< Updated upstream
	// ###################OPTIMISTIC LOCK#################################

	newSenderBalance := senderWalletId.Balance.Sub(req.Amount)
	err = s.wallRepo.UpdateBalanceTx(ctx, tx, senderWalletId.ID, newSenderBalance, int64(senderWalletId.Version))
	if err != nil {
		return nil, customErr.NewAppError(http.StatusInternalServerError, "SENDER_WALLET_UPDATE_FAILED", "Failed to update sender wallet balance")
	}
	//credit +ve amount
	newReceiverBalance := receiverWalletId.Balance.Add(req.Amount)
	err = s.wallRepo.UpdateBalanceTx(ctx, tx, receiverWalletId.ID, newReceiverBalance, int64(receiverWalletId.Version))
	if err != nil {
=======

	// ###################OPTIMISTIC LOCK#################################

	logger.Info(ctx, "debiting sender wallet", "wallet_id", senderWalletId.ID, "amount", req.Amount.Neg(), "expected_version", senderWalletId.Version)
	err = s.wallRepo.UpdateBalanceTx(ctx, tx, senderWalletId.ID, req.Amount.Neg(), int64(senderWalletId.Version))
	if err != nil {
		logger.Error(ctx, "sender wallet update failed", "wallet_id", senderWalletId.ID, "error", err)
		return nil, customErr.NewAppError(http.StatusInternalServerError, "SENDER_WALLET_UPDATE_FAILED", "Failed to update sender wallet balance")
	}
	//credit +ve amount
	logger.Info(ctx, "crediting receiver wallet", "wallet_id", receiverWalletId.ID, "amount", req.Amount, "expected_version", receiverWalletId.Version)
	err = s.wallRepo.UpdateBalanceTx(ctx, tx, receiverWalletId.ID, req.Amount, int64(receiverWalletId.Version))
	if err != nil {
		logger.Error(ctx, "receiver wallet update failed", "wallet_id", receiverWalletId.ID, "error", err)
>>>>>>> Stashed changes
		return nil, customErr.NewAppError(http.StatusInternalServerError, "RECEIVER_WALLET_UPDATE_FAILED", "Failed to update receiver wallet balance")
	}

	// also create a transaction record in the transaction table and ledger entries(two rows entry (debit for sender, and credit for receiver)) in the ledger table for both sender and receiver with requested amount
	transaction := &txmodel.Transaction{
		ID:               uuid.New().String(), // generate a new uuid for the transaction
		SenderWalletID:   &senderWalletId.ID,
		ReceiverWalletID: receiverWalletId.ID,
		Amount:           req.Amount,
		Description:      req.Description,
		IdempotencyKey:   req.IdempotencyKey,
		Status:           "success",
<<<<<<< Updated upstream
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
=======
		CreatedAt:        time.Now().UTC(),
>>>>>>> Stashed changes
	}
	//create the transaction record in the transaction table using the CreateTx method of the transaction repository
	err = s.txRepo.CreateTx(ctx, transaction, tx)
	if err != nil {
<<<<<<< Updated upstream
=======
		logger.Error(ctx, "transaction record creation failed", "transaction_id", transaction.ID, "error", err)
>>>>>>> Stashed changes
		return nil, customErr.ErrInternalServer
	}

	// create two ledger rows (debit for sender, and credit for receiver) in ledger table using the CreateTx method of the ledger repository
	// create a ledger entry for the sender (debit)

	senderDebitEntry := &ledgerModel.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      senderWalletId.ID,
		TransactionID: transaction.ID,
		Amount:        req.Amount.Neg(), // negative amount for debit
		EntryType:     "debit",
		CreatedAt:     time.Now().UTC(),
	}
	//commit the ledger entry for the sender
	err = s.ledgerRepo.CreateTx(ctx, senderDebitEntry, tx)
	if err != nil {
<<<<<<< Updated upstream
=======
		logger.Error(ctx, "sender ledger entry creation failed", "ledger_entry_id", senderDebitEntry.ID, "transaction_id", transaction.ID, "error", err)
>>>>>>> Stashed changes
		return nil, customErr.ErrInternalServer
	}

	// create a ledger entry for the receiver (credit)
	receiverCreditEntry := &ledgerModel.LedgerEntry{
		ID:            uuid.New().String(),
		WalletID:      receiverWalletId.ID,
		TransactionID: transaction.ID,
		Amount:        req.Amount, // positive amount for credit
		EntryType:     "credit",
		CreatedAt:     time.Now().UTC(),
	}
	//commit the ledger entry for the receiver
	err = s.ledgerRepo.CreateTx(ctx, receiverCreditEntry, tx)
	if err != nil {
<<<<<<< Updated upstream
=======
		logger.Error(ctx, "receiver ledger entry creation failed", "ledger_entry_id", receiverCreditEntry.ID, "transaction_id", transaction.ID, "error", err)
>>>>>>> Stashed changes
		return nil, customErr.ErrInternalServer
	}
	// commmit the db transaction
	err = tx.Commit()
	if err != nil {
<<<<<<< Updated upstream
		return nil, customErr.ErrInternalServer
	}
	// return the transaction record
	return transaction, nil

=======
		logger.Error(ctx, "transfer transaction commit failed", "transaction_id", transaction.ID, "error", err)
		return nil, customErr.ErrInternalServer
	}
	logger.Info(ctx, "transfer completed", "transaction_id", transaction.ID, "sender_wallet_id", senderWalletId.ID, "receiver_wallet_id", receiverWalletId.ID, "amount", req.Amount)
	// return the transaction record
	return transaction, nil

}

func (s *transactionService) GetHistory(ctx context.Context, userID string, params pgnDto.PaginationParams) ([]txmodel.Transaction, *pgnDto.PaginationMeta, error) {
	// Validation - FIX: add minimum validation
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit <= 0 {
		params.Limit = 10 // default
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	wallet, err := s.wallRepo.GetByUserID(ctx, userID)
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

	meta := &pgnDto.PaginationMeta{
		Page:      params.Page,
		Limit:     params.Limit,
		Total:     total,
		TotalPage: totalPages,
	}

	return txs, meta, nil
>>>>>>> Stashed changes
}
