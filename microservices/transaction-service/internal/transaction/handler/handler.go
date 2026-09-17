package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/shared/pagination"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/dto"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/service"
	"github.com/shopspring/decimal"
)

type TransactionHandler struct {
	svc service.TransactionService
}

func NewTransactionHandler(svc service.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

func (h *TransactionHandler) Transfer(c *gin.Context) {
	senderUserID, exist := c.Get("userID")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User context not found"))
		c.Abort()
		return
	}
	senderIDStr, ok := senderUserID.(string)
	if !ok || senderIDStr == "" {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		c.Abort()
		return
	}

	var req dto.TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error()))
		return
	}
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_AMOUNT", "Amount must be greater than zero"))
		return
	}

	tx, err := h.svc.Transfer(c.Request.Context(), senderIDStr, req)
	if err != nil {
		c.Error(err)
		c.Abort()
		return
	}

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
	if !ok || userIDStr == "" {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		return
	}

	var params pagination.Params
	if err := c.ShouldBindQuery(&params); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	txs, meta, err := h.svc.GetHistory(c.Request.Context(), userIDStr, params)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, pagination.Response{
		Success: true,
		Data:    txs,
		Meta:    *meta,
	})
}
