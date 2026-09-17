package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/dto"
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/service"
)

// InternalHandler exposes the inter-service API. These routes are not proxied
// through the API gateway and are only reachable on the internal network.
type InternalHandler struct {
	mutationSvc service.MutationService
	walletSvc   service.WalletService
}

func NewInternalHandler(mutationSvc service.MutationService, walletSvc service.WalletService) *InternalHandler {
	return &InternalHandler{mutationSvc: mutationSvc, walletSvc: walletSvc}
}

// CreateWallet is called by user-service when it registers a new user.
func (h *InternalHandler) CreateWallet(c *gin.Context) {
	var req dto.CreateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	w, err := h.mutationSvc.CreateWallet(c.Request.Context(), req.UserID, req.Currency)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"wallet":  w,
	})
}

// AdjustBalance applies a signed delta to a wallet. Used by transaction-service
// and payment-service (top-ups) for debit/credit operations.
func (h *InternalHandler) AdjustBalance(c *gin.Context) {
	var req dto.AdjustBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	w, err := h.mutationSvc.AdjustBalance(c.Request.Context(), req.WalletID, req.Amount, req.ExpectedVersion)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"wallet":  w,
	})
}

// ResolveByUserID resolves a user's wallet so other services can look up
// sender/receiver wallets without owning the wallets table.
func (h *InternalHandler) ResolveByUserID(c *gin.Context) {
	userID := c.Param("userID")
	if userID == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", "userID is required"))
		return
	}

	w, err := h.walletSvc.GetWalletByUserID(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"wallet":  w,
	})
}
