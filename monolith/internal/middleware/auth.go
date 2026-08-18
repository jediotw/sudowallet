package middleware

import (
	"github.com/gin-gonic/gin"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"net/http"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		//check req header first if it has authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Error(customErr.NewAppError(http.StatusUnauthorized, "Missing Authorization Header", "Authorization header is required"))
			c.Abort()
		}
		//else it have authorization header so we need to verify the token
		//split the header to get the token as header have the token in the format of "Bearer <token>" also split Breaker and token  so we cab get the token only

	}
}
