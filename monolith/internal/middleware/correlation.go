package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
)

func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		corID := c.GetHeader("X-Correlation-ID")
		if corID == "" {
			//generate a new correlation ID if not present in the request header
			corID = uuid.New().String()
		}
		//set in the request context so that it can be used in the logger
		//Creates a child context carrying request-scoped data
		//parent context is c.Request.Context(), and the key-value pair is logger.CorrelationIDKey and corID
		var parentCtx context.Context = c.Request.Context()
		Newctx := context.WithValue(parentCtx, logger.CorrelationIDKey, corID)

		//ATTACH the Newctx context to an HTTP request
		c.Request = c.Request.WithContext(Newctx)
		//continue to the next middleware or handler in the chain
		c.Next()
	}
}
