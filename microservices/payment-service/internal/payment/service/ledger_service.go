package service

import (
	"context"

	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/model"
	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/repository"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/shopspring/decimal"
)

type LedgerService interface {
	GetMutationHistory(ctx context.Context, userID string) ([]*model.LedgerEntry, error)
	ReconcileWalletBalance(ctx context.Context, userID string) (bool, decimal.Decimal, decimal.Decimal, error)
}

type ledgerService struct {
	ledRepo    repository.LedgerRepository
	walletRepo repository.WalletRepository
}

func NewLedgerService(lRepo repository.LedgerRepository, wRepo repository.WalletRepository) LedgerService {
	return &ledgerService{ledRepo: lRepo, walletRepo: wRepo}
}

// GetMutationHistory returns every ledger entry that changed the user's wallet.
func (s *ledgerService) GetMutationHistory(ctx context.Context, userID string) ([]*model.LedgerEntry, error) {
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "ledger: failed to resolve wallet for user", "user_id", userID, "error", err)
		return nil, customErr.ErrWalletNotFound
	}
	return s.ledRepo.GetEntriesByWalletID(ctx, wallet.ID)
}

// ReconcileWalletBalance compares the stored wallet balance with the balance
// recomputed from the ledger and reports whether they agree.
func (s *ledgerService) ReconcileWalletBalance(ctx context.Context, userID string) (bool, decimal.Decimal, decimal.Decimal, error) {
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "ledger: failed to resolve wallet for user", "user_id", userID, "error", err)
		return false, decimal.Zero, decimal.Zero, customErr.ErrWalletNotFound
	}

	ledgerBalance, err := s.ledRepo.GetBalanceByWalletID(ctx, wallet.ID)
	if err != nil {
		logger.Error(ctx, "ledger: failed to compute ledger balance", "wallet_id", wallet.ID, "error", err)
		return false, wallet.Balance, decimal.Zero, customErr.ErrInternalServer
	}

	isConsistent := wallet.Balance.Equal(ledgerBalance)
	return isConsistent, wallet.Balance, ledgerBalance, nil
}
