package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	pgnDto "github.com/saurabhkr78/sudowallet/monolith/internal/common/dto"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
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

	// Get the sender user ID from the context.
	// The authentication middleware should have set this value.
	senderUserID, exist := c.Get("userID")

	// If the user ID is not present, the request is unauthorized.
	if !exist {
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
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		return
	}

	var req dto.TransferRequest

	// Read the JSON request body and bind it to TransferRequest.
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_REQUEST_BODY",
			"Invalid request body",
		))
		return
	}

	// Amount must be greater than zero.
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		c.Error(customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_AMOUNT",
			"Amount must be greater than zero",
		))
		return
	}

	// Description is required.
	if req.Description == "" {
		c.Error(customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_DESCRIPTION",
			"Description cannot be empty",
		))
		return
	}

	// Call the service layer to perform the transfer.
	parentcontext := c.Request.Context()
	tx, err := h.svc.Transfer(
		parentcontext,
		senderIDStr,
		req,
	)

	if err != nil {
		// Pass the service error to the error middleware and stop the request
		// immediately so Gin does not continue with a successful response.
		c.Error(err)
		c.Abort()
		return
	}

	// Send the successful transfer response.
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Transfer successful",
		"data":    tx,
	})
}

func (h *TransactionHandler) GetHistory(c *gin.Context) {
	userID, exist := c.Get("userID")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User context not found"))
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		return
	}

	var params pgnDto.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	txs, meta, err := h.svc.GetHistory(c.Request.Context(), userIDStr, params)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, pgnDto.PaginatedResponse{
		Success: true,
		Data:    txs,
		Meta:    *meta,
	})
}
