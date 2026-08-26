package service

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	walletModel "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/model"
	walletRepo "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/repository"
	"time"
)

/*
WalletService defines what wallet operations are available, walletService stores the repository it needs, and NewWalletService() injects that repository into the service.
*/
// all the wallet operations?
type WalletService interface {
	GetWalletByUserID(ctx context.Context, userID string) (*walletModel.Wallet, error)
}

// store the wallet repository in the service struct so that we can use it in the service methods
type walletService struct {
	walletRepo  walletRepo.WalletRepository
	redisClient *redis.Client
}

// constructore to create a new wallet service and inject the wallet repository into it
func NewWalletService(wRepo walletRepo.WalletRepository, rdb *redis.Client) WalletService {
	return &walletService{walletRepo: wRepo, redisClient: rdb}
}

// implement the GetWalletByUserID method of the WalletService interface, this method will call the GetByUserID method of the wallet repository to get the wallet by user id
// this is the method attached to the walletService struct that implements the WalletService interface, it takes a context and a user id as parameters and returns a wallet model and an error
func (s *walletService) GetWalletByUserID(ctx context.Context, userID string) (*walletModel.Wallet, error) {

	key := "wallet:" + userID

	// 1. Try Redis
	walletData, err := s.redisClient.Get(ctx, key).Result()

	if err == nil {
		// CACHE HIT

		var wallet walletModel.Wallet

		if err := json.Unmarshal([]byte(walletData), &wallet); err != nil {
			//I expected this key to contain a valid Wallet JSON. If it doesn't, don't keep serving a known-bad cache entry. Delete it and reconstruct it from DB."
			// Cache contains corrupted/invalid data.
			// Delete it and go to DB.
			_ = s.redisClient.Del(ctx, key).Err()
		} else {
			return &wallet, nil
		}
	}

	// 2. Cache miss OR Redis failure
	if err != nil && err != redis.Nil {
		logger.Warn(
			ctx,
			"Redis unavailable, falling back to database",
			"error", err,
		)
	}

	// 3. DB call
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, customErr.NewAppError(
			500,
			"WALLET_NOT_FOUND",
			"Wallet not found",
		)
	}

	// 4. Serialize
	data, err := json.Marshal(wallet)
	if err != nil {
		return nil, customErr.NewAppError(
			500,
			"WALLET_MARSHAL_ERROR",
			"Failed to marshal wallet",
		)
	}

	// 5. Populate cache
	if err := s.redisClient.Set(
		ctx,
		key,
		data,
		5*time.Minute,
	).Err(); err != nil {
		// Usually DON'T fail the request just because caching failed.
		logger.Warn(
			ctx,
			"Failed to populate wallet cache",
			"error", err,
		)
	}

	return wallet, nil
}
