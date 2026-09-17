package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/service"
)

type WalletHandler struct {
	walletSvc service.WalletService
}

func NewWalletHandler(walletSvc service.WalletService) *WalletHandler {
	return &WalletHandler{walletSvc: walletSvc}
}

func (h *WalletHandler) GetWalletByUserID(c *gin.Context) {
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

	wallet, err := h.walletSvc.GetWalletByUserID(c.Request.Context(), userIDStr)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"wallet":  wallet,
	})
}
