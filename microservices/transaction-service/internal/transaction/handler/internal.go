package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/repository"
)

// InternalHandler exposes the inter-service API used by payment-service. These
// routes are not proxied through the API gateway.
type InternalHandler struct {
	txRepo     repository.TransactionRepository
	ledgerRepo repository.LedgerRepository
}

func NewInternalHandler(txRepo repository.TransactionRepository, ledgerRepo repository.LedgerRepository) *InternalHandler {
	return &InternalHandler{txRepo: txRepo, ledgerRepo: ledgerRepo}
}

// GetTransactionsByDate serves payment-service's daily transaction report.
func (h *InternalHandler) GetTransactionsByDate(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", "from and to query params are required"))
		return
	}

	start, err := time.Parse(time.RFC3339, from)
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", "from must be RFC3339"))
		return
	}
	end, err := time.Parse(time.RFC3339, to)
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", "to must be RFC3339"))
		return
	}

	txs, err := h.txRepo.GetTransactionsByDate(c.Request.Context(), start, end)
	if err != nil {
		logger.Error(c.Request.Context(), "internal: transactions by date failed", "error", err)
		c.Error(customErr.ErrInternalServer)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    txs,
	})
}

// GetLedgerBalance serves payment-service's per-wallet reconciliation.
func (h *InternalHandler) GetLedgerBalance(c *gin.Context) {
	walletID := c.Param("walletID")
	if walletID == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", "walletID is required"))
		return
	}

	balance, err := h.ledgerRepo.GetBalanceByWalletID(c.Request.Context(), walletID)
	if err != nil {
		logger.Error(c.Request.Context(), "internal: ledger balance lookup failed", "wallet_id", walletID, "error", err)
		c.Error(customErr.ErrInternalServer)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"balance": balance,
	})
}

// GetLedgerEntries serves payment-service's mutations listing.
func (h *InternalHandler) GetLedgerEntries(c *gin.Context) {
	walletID := c.Param("walletID")
	if walletID == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", "walletID is required"))
		return
	}

	entries, err := h.ledgerRepo.GetEntriesByWalletID(c.Request.Context(), walletID)
	if err != nil {
		logger.Error(c.Request.Context(), "internal: ledger entries lookup failed", "wallet_id", walletID, "error", err)
		c.Error(customErr.ErrInternalServer)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entries,
	})
}