package database

import (
	"context"
	"github.com/redis/go-redis/v9"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	"time"
)

func ConnectRedis(address string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: address,
	})
	//check the connection using context
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	//PING THE REDIS SERVER
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	//SUCCESSFULLY CONNECTED TO REDIS
	logger.Log.Info("Successfully connected to Redis at %s", "address", address)
	return rdb, nil
}
