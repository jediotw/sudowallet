package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/model"
)

// walletCacheTTL matches the balance staleness window the monolith used.
const walletCacheTTL = 5 * time.Minute

// WalletCacheKey returns the canonical Redis key for a user's cached wallet.
//
// NOTE: the monolith wrote the cache under "wallet:<userID>" but invalidated it
// under "wallet:user:<userID>". wallet-service uses "wallet:user:<userID>"
// everywhere so invalidation and reads agree.
func WalletCacheKey(userID string) string {
	return "wallet:user:" + userID
}

type WalletCache interface {
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, bool, error)
	SetByUserID(ctx context.Context, userID string, w *model.Wallet) error
	Invalidate(ctx context.Context, userID string) error
}

type redisWalletCache struct {
	rdb *redis.Client
}

func NewWalletCache(rdb *redis.Client) WalletCache {
	return &redisWalletCache{rdb: rdb}
}

// GetByUserID returns the cached wallet and hit=true on a cache hit.
// A miss or a Redis failure returns hit=false so the caller falls back to the DB.
func (c *redisWalletCache) GetByUserID(ctx context.Context, userID string) (*model.Wallet, bool, error) {
	key := WalletCacheKey(userID)

	data, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		logger.Warn(ctx, "wallet cache read failed, falling back to database", "key", key, "error", err)
		return nil, false, nil
	}

	var w model.Wallet
	if err := json.Unmarshal([]byte(data), &w); err != nil {
		// Corrupt cache entry: drop it and fall back to the DB.
		logger.Warn(ctx, "wallet cache held corrupt data, discarding", "key", key, "error", err)
		_ = c.rdb.Del(ctx, key).Err()
		return nil, false, nil
	}

	return &w, true, nil
}

func (c *redisWalletCache) SetByUserID(ctx context.Context, userID string, w *model.Wallet) error {
	data, err := json.Marshal(w)
	if err != nil {
		return err
	}

	return c.rdb.Set(ctx, WalletCacheKey(userID), data, walletCacheTTL).Err()
}

func (c *redisWalletCache) Invalidate(ctx context.Context, userID string) error {
	if err := c.rdb.Del(ctx, WalletCacheKey(userID)).Err(); err != nil {
		logger.Warn(ctx, "wallet cache invalidation failed", "user_id", userID, "error", err)
		return err
	}
	return nil
}
