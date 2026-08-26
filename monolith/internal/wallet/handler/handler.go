package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/saurabhkr78/sudowallet/monolith/internal/wallet/service"
)

type WalletHandler struct {
	svc service.WalletService
}

func NewWalletHandler(svc service.WalletService) *WalletHandler {
	return &WalletHandler{
		svc: svc,
	}
}
func (h *WalletHandler) GetWalletByUserID(c *gin.Context) {
	//get the user id from the url parameter and call the service method to get the wallet by user id
	userID := c.GetString("userID")                                     //take out the userID from the url parameter c.getstring("userID"):get the user id from the url parameter
	wallet, err := h.svc.GetWalletByUserID(c.Request.Context(), userID) //call the service method to get the wallet by user id
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "wallet": wallet})
}
