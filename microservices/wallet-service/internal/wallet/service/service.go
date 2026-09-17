package service

import (
	"context"

	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/model"
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/repository"
)

type WalletService interface {
	GetWalletByUserID(ctx context.Context, userID string) (*model.Wallet, error)
	// GetWalletByUserIDFresh reads the wallet straight from the database,
	// bypassing the cache. Internal transfers need the accurate version and
	// balance for optimistic concurrency control.
	GetWalletByUserIDFresh(ctx context.Context, userID string) (*model.Wallet, error)
	GetAllWallets(ctx context.Context) ([]*model.Wallet, error)
}

type walletService struct {
	walletRepo repository.WalletRepository
	cache      repository.WalletCache
}

func NewWalletService(walletRepo repository.WalletRepository, cache repository.WalletCache) WalletService {
	return &walletService{walletRepo: walletRepo, cache: cache}
}

func (s *walletService) GetWalletByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	logger.Info(ctx, "wallet service lookup started", "user_id", userID)

	// 1. Try Redis cache.
	if w, hit, err := s.cache.GetByUserID(ctx, userID); err == nil && hit && w != nil {
		logger.Info(ctx, "wallet service cache hit", "user_id", userID, "wallet_id", w.ID)
		return w, nil
	}

	// 2. Cache miss or Redis unavailable: read from the database.
	w, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 3. Populate the cache. Never fail the request just because caching failed.
	if err := s.cache.SetByUserID(ctx, userID, w); err != nil {
		logger.Warn(ctx, "failed to populate wallet cache", "user_id", userID, "error", err)
	}

	return w, nil
}

// GetAllWallets returns every non-deleted wallet. Used by payment-service for
// full reconciliation.
func (s *walletService) GetAllWallets(ctx context.Context) ([]*model.Wallet, error) {
	wallets, err := s.walletRepo.ListAll(ctx)
	if err != nil {
		logger.Error(ctx, "wallet service list all failed", "error", err)
		return nil, err
	}
	return wallets, nil
}

// GetWalletByUserIDFresh returns the wallet directly from the database,
// including the current version, for internal callers that mutate balances.
func (s *walletService) GetWalletByUserIDFresh(ctx context.Context, userID string) (*model.Wallet, error) {
	return s.walletRepo.GetByUserID(ctx, userID)
}
