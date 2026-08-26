package middleware

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	auth "github.com/saurabhkr78/sudowallet/monolith/internal/auth"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"net/http"
	"strings"
)

func AuthMiddleware(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {

		// First, check whether the request contains an Authorization header.
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.Error(customErr.NewAppError(
				http.StatusUnauthorized,
				"Missing Authorization Header",
				"Authorization header is required",
			))
			c.Abort()
			return
		}

		// The Authorization header should have the format:
		// "Bearer <token>"
		//
		// Split the header into separate parts using whitespace.
		// Example:
		// "Bearer abc123" -> ["Bearer", "abc123"]
		parts := strings.Fields(authHeader)

		// We expect exactly two parts:
		// parts[0] -> "Bearer"
		// parts[1] -> the actual JWT token
		//
		// If there are not exactly two parts, or the first part is not
		// "Bearer", then the Authorization header is invalid.
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Error(customErr.NewAppError(
				http.StatusUnauthorized,
				"Invalid Authorization Header",
				"Authorization header must use Bearer scheme",
			))
			c.Abort()
			return
		}

		// The second part is the actual JWT string.
		tokenString := parts[1]

		//#############################redis token blacklist check#############################

		//check if the token is blacklisted in Redis
		blacklistKey := fmt.Sprintf("blacklist:%s", tokenString)
		exists, err := rdb.Exists(c.Request.Context(), blacklistKey).Result()
		if err == nil && exists > 0 {
			c.Error(customErr.NewAppError(
				http.StatusUnauthorized,
				"Token REVOKED",
				"Login senssion has expired, please login again",
			))
			c.Abort()
			return
		}
		// Now verify the JWT using the extracted token string and extract the claims.
		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			c.Error(customErr.NewAppError(
				http.StatusUnauthorized,
				"Invalid Token",
				"Token validation failed",
			))
			c.Abort()
			return

		}
		//save to the context that the user is authenticated
		//get JWT claims
		//extract the user id from the claims and set it in the context for use in subsequent handlers
		c.Set("userID", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("token_string", tokenString) //for logout purpose, we need to store the token string in the context so that we can blacklist it in redis when the user logs out

		// If the token is valid, proceed to the next handler.
		//Handler can access authenticated user
		c.Next()

	}
}
