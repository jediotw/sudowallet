package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/service"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
)

type LedgerHandler struct {
	svc service.LedgerService
}

func NewLedgerHandler(svc service.LedgerService) *LedgerHandler {
	return &LedgerHandler{svc: svc}
}

// GetMutations returns every ledger entry (credits/debits) for the caller's wallet.
func (h *LedgerHandler) GetMutations(c *gin.Context) {
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

	entries, err := h.svc.GetMutationHistory(c.Request.Context(), userIDStr)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entries,
	})
}

// Reconcile checks whether the wallet balance matches the ledger-computed balance.
func (h *LedgerHandler) Reconcile(c *gin.Context) {
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

	isConsistent, walletBalance, calculatedBalance, err := h.svc.ReconcileWalletBalance(c.Request.Context(), userIDStr)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"is_consistent":      isConsistent,
		"wallet_balance":     walletBalance,
		"calculated_balance": calculatedBalance,
	})
}
