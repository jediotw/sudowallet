package middleware

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"net/http"
	"time"
)

// instead of pipeline we use lua script to implement rate limiting, because pipeline is not atomic
var rateLimitScript = redis.NewScript(`
	local count = redis.call("INCR", KEYS[1])

	if count == 1 then
		redis.call("EXPIRE", KEYS[1], ARGV[1])
	end

	return count
`)

func RateLimit(
	redisClient *redis.Client,
	limit int,
	window time.Duration,
) gin.HandlerFunc {

	return func(c *gin.Context) {

		userIP := c.ClientIP()

		// Fixed-window ID
		currentWindow := time.Now().Unix() / int64(window.Seconds())

		key := fmt.Sprintf(
			"rate_limit:%s:%d",
			userIP,
			currentWindow,
		)

		ctx := c.Request.Context()

		count, err := rateLimitScript.Run(
			ctx,
			redisClient,
			[]string{key},
			int(window.Seconds()),
		).Int64()

		if err != nil {
			c.Error(customErr.ErrInternalServer)
			c.Abort()
			return
		}

		if count > int64(limit) {
			c.Error(
				customErr.NewAppError(
					http.StatusTooManyRequests,
					"Too many requests. Please try again later.",
					"rate limit exceeded",
				),
			)
			c.Abort()
			return
		}

		c.Next()
	}
}
