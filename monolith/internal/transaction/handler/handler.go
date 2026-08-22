package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	"github.com/saurabhkr78/sudowallet/monolith/internal/transaction/dto"
	"github.com/saurabhkr78/sudowallet/monolith/internal/transaction/service"
	"github.com/shopspring/decimal"
)

type TransactionHandler struct {
	svc service.TransactionService
}

func NewTransactionHandler(s service.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: s}
}

func (h *TransactionHandler) Transfer(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "transfer request received", "method", c.Request.Method, "path", c.Request.URL.Path)

	// Get the sender user ID from the context.
	// The authentication middleware should have set this value.
	senderUserID, exist := c.Get("userID")

	// If the user ID is not present, the request is unauthorized.
	if !exist {
		logger.Warn(ctx, "transfer rejected: user context missing")
		c.Error(customErr.NewAppError(
			http.StatusUnauthorized,
			"UNAUTHORIZED",
			"User context not found",
		))
		return
	}

	// Make sure the value stored in the context is actually a string.
	senderIDStr, ok := senderUserID.(string)
	if !ok {
		logger.Warn(ctx, "transfer rejected: invalid user context")
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		return
	}

	var req dto.TransferRequest

	// Read the JSON request body and bind it to TransferRequest.
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn(ctx, "transfer rejected: request body binding failed", "error", err)
		c.Error(customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_REQUEST_BODY",
			"Invalid request body",
		))
		return
	}

	// Amount must be greater than zero.
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		logger.Warn(ctx, "transfer rejected: invalid amount", "sender_user_id", senderIDStr, "amount", req.Amount)
		c.Error(customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_AMOUNT",
			"Amount must be greater than zero",
		))
		return
	}

	// Description is required.
	if req.Description == "" {
		logger.Warn(ctx, "transfer rejected: description missing", "sender_user_id", senderIDStr)
		c.Error(customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_DESCRIPTION",
			"Description cannot be empty",
		))
		return
	}

	// Call the service layer to perform the transfer.
	tx, err := h.svc.Transfer(
		ctx,
		senderIDStr,
		req,
	)

	if err != nil {
		logger.Error(ctx, "transfer request failed", "sender_user_id", senderIDStr, "receiver_email", req.ReceiverEmail, "error", err)
		// Pass the service error to the error middleware and stop the request
		// immediately so Gin does not continue with a successful response.
		c.Error(err)
		c.Abort()
		return
	}
	logger.Info(ctx, "transfer request completed", "sender_user_id", senderIDStr, "transaction_id", tx.ID)

	// Send the successful transfer response.
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Transfer successful",
		"data":    tx,
	})
}
