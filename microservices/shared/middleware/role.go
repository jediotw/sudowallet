package middleware

import (
	"github.com/gin-gonic/gin"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"net/http"
)

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the role from AuthMiddleware
		userRole, exists := c.Get("role")

		if !exists {
			c.Error(customErr.NewAppError(http.StatusForbidden, "ACCESS_DENIED", "You don't have access to this api."))
			c.Abort()
			return
		}

		// Check whether user role is registered in allowedRoles
		roleStr, ok := userRole.(string)
		if !ok {
			c.Error(customErr.NewAppError(http.StatusForbidden, "ACCESS_DENIED", "Invalid user role format."))
			c.Abort()
			return
		}
		isAllowed := false
		for _, role := range allowedRoles {
			if roleStr == role {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.Error(customErr.NewAppError(http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "You don't have permission to this api."))
			c.Abort()
			return
		}

		c.Next()
	}
}
