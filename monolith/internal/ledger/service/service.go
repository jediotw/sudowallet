package service

import (
	"context"
	"github.com/saurabhkr78/sudowallet/monolith/internal/ledger/model"
	ledgerRepo "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/repository"
	walletRepo "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/repository"
	"github.com/shopspring/decimal"
)

type LedgerService interface {
	//"Show me the history of all transactions that changed this wallet's balance."
	//so we need wallet repository dependency composition
	//This is basically a transaction history / statement API.
	//It could internally call:ledgerRepo.GetEntriesByWalletID(ctx, walletID)
	GetMutationHistory(ctx context.Context, walletID string) ([]*model.LedgerEntry, error)

	// Check whether the balance stored in the wallet matches the balance that the ledger says it should have
	// so supoose the wallet balance is 100 and the ledger says it should be 100(by debit and credit ), then the wallet is consistent with the ledger.
	//if there is something fishy then this api can detect it and return false. This is basically a reconciliation API.
	// This is basically a reconciliation API.
	// It could internally call:ledgerRepo.GetBalanceByWalletID(ctx, walletID)
	ReconcileWalletBalance(ctx context.Context, userID string) (bool, decimal.Decimal, decimal.Decimal, error)
}
type ledgerService struct {
	ledRepo  ledgerRepo.LedgerRepository
	wallRepo walletRepo.WalletRepository
}

func NewLedgerService(lRepo ledgerRepo.LedgerRepository, wRepo walletRepo.WalletRepository) LedgerService {
	return &ledgerService{
		ledRepo:  lRepo,
		wallRepo: wRepo,
	}
}
func (s *ledgerService) GetMutationHistory(ctx context.Context, userID string) ([]*model.LedgerEntry, error) {
	//get the wallet id for the given user id from the wallet repository
	user, err := s.wallRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	//find the wallet id by wallet id and get the ledger entries for that wallet id from the ledger repository
	return s.ledRepo.GetEntriesByWalletID(ctx, user.ID)

}

// return the true if the wallet balance is equal to the ledger balance for the given wallet id, else return false
// also return an error if there is any error while getting the wallet balance or the ledger balance
// also return the wallet balance and the ledger balance for the given wallet id
func (s *ledgerService) ReconcileWalletBalance(ctx context.Context, userID string) (bool, decimal.Decimal, decimal.Decimal, error) {
	//1.so get the wallet id for the given user id from the wallet repository
	//this GetByUserID method returs the wallet model for the given user id, which contains the wallet id and the wallet balance
	user, err := s.wallRepo.GetByUserID(ctx, userID)
	if err != nil {
		return false, decimal.Zero, decimal.Zero, err
	}
	//get the wallet balance from the returned wallet model and set it into the walletBalance variable
	walletBalance := user.Balance

	//2.get the ledger balance for the given wallet id from the ledger repository
	ledgerBalance, err := s.ledRepo.GetBalanceByWalletID(ctx, user.ID)
	if err != nil {
		//if err is not nil then return false, walletBalance, decimal.Zero(zero) and the error
		return false, walletBalance, decimal.Zero, err
	}
	//3.else balance is available, so compare the wallet balance and the ledger balance and return true if they are equal else return false
	isConsistent := walletBalance == ledgerBalance

	return isConsistent, walletBalance, ledgerBalance, nil
}
