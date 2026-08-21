package service

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
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
	// check if this transaction is already processed by checking the idempotency key in the transaction table
	existing, _ := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if existing != nil {
		//if the transaction is already processed then return the existing transaction
		return existing, nil
	}
	// find receiver by email it returns the user model which contains the user id and other details
	receiver, err := s.userRepo.GetByEmail(ctx, req.ReceiverEmail)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "RECEIVER_NOT_FOUND", "Receiver not found")
	}
	// start db transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	defer tx.Rollback() // rollback the transaction if any error occurs

	// look for sender and receivver wallet by their ids
	senderWalletId, err := s.wallRepo.GetByUserID(ctx, senderUserID)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "SENDER_WALLET_NOT_FOUND", "Sender wallet not found")
	}
	receiverWalletId, err := s.wallRepo.GetByUserID(ctx, receiver.ID)
	if err != nil {
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
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	//create the transaction record in the transaction table using the CreateTx method of the transaction repository
	err = s.txRepo.CreateTx(ctx, transaction, tx)
	if err != nil {
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
		return nil, customErr.ErrInternalServer
	}
	// commmit the db transaction
	err = tx.Commit()
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	// return the transaction record
	return transaction, nil

}
