package service

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/model"
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/repository"
	"github.com/shopspring/decimal"
)

const defaultCurrency = "IDR"

type MutationService interface {
	CreateWallet(ctx context.Context, userID string, currency string) (*model.Wallet, error)
	// AdjustBalance applies a signed delta: positive amount credits, negative debits.
	// expectedVersion provides optimistic concurrency control.
	AdjustBalance(ctx context.Context, walletID string, amount decimal.Decimal, expectedVersion int64) (*model.Wallet, error)
}

type mutationService struct {
	walletRepo repository.WalletRepository
	cache      repository.WalletCache
}

func NewMutationService(walletRepo repository.WalletRepository, cache repository.WalletCache) MutationService {
	return &mutationService{walletRepo: walletRepo, cache: cache}
}

func (s *mutationService) CreateWallet(ctx context.Context, userID string, currency string) (*model.Wallet, error) {
	logger.Info(ctx, "wallet creation started", "user_id", userID, "currency", currency)

	// Guard against double-provisioning (users.user_id is UNIQUE, so the DB
	// would reject this anyway; fail fast with a clear error).
	existing, err := s.walletRepo.GetByUserID(ctx, userID)
	if err == nil && existing != nil {
		logger.Warn(ctx, "wallet creation rejected: already exists", "user_id", userID)
		return nil, customErr.NewAppError(http.StatusConflict, "WALLET_ALREADY_EXISTS", "Wallet already exists for this user.")
	}

	if currency == "" {
		currency = defaultCurrency
	}

	now := time.Now()
	w := &model.Wallet{
		ID:        uuid.New().String(),
		UserID:    userID,
		Balance:   decimal.Zero,
		Currency:  currency,
		Status:    "active",
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.walletRepo.Create(ctx, w); err != nil {
		logger.Error(ctx, "wallet creation failed", "user_id", userID, "error", err)
		return nil, customErr.ErrInternalServer
	}

	logger.Info(ctx, "wallet creation completed", "user_id", userID, "wallet_id", w.ID)
	return w, nil
}

func (s *mutationService) AdjustBalance(ctx context.Context, walletID string, amount decimal.Decimal, expectedVersion int64) (*model.Wallet, error) {
	logger.Info(ctx, "wallet balance adjustment started", "wallet_id", walletID, "amount", amount, "expected_version", expectedVersion)

	if amount.IsZero() {
		return nil, customErr.NewAppError(http.StatusBadRequest, "INVALID_AMOUNT", "Amount must be non-zero.")
	}

	// Resolve the owner up front so we can invalidate the right cache key.
	w, err := s.walletRepo.GetByID(ctx, walletID)
	if err != nil {
		return nil, err
	}

	if err := s.walletRepo.UpdateBalance(ctx, walletID, amount, expectedVersion); err != nil {
		return nil, err
	}

	// Balance changed: drop the cached copy so the next read is fresh.
	if err := s.cache.Invalidate(ctx, w.UserID); err != nil {
		logger.Warn(ctx, "failed to invalidate wallet cache after adjustment", "user_id", w.UserID, "error", err)
	}

	updated, err := s.walletRepo.GetByID(ctx, walletID)
	if err != nil {
		logger.Error(ctx, "wallet reload after adjustment failed", "wallet_id", walletID, "error", err)
		return nil, err
	}

	logger.Info(ctx, "wallet balance adjustment completed", "wallet_id", walletID, "new_balance", updated.Balance)
	return updated, nil
}
