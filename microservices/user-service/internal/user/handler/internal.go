package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/service"
)

// InternalHandler exposes the inter-service API. These routes are not proxied
// through the API gateway and are only reachable on the internal network.
type InternalHandler struct {
	svc *service.UserService
}

func NewInternalHandler(svc *service.UserService) *InternalHandler {
	return &InternalHandler{svc: svc}
}

// VerifyCredentials is called by auth-service to validate an email/password pair
// on login. Ownership of the password hash stays with user-service.
func (h *InternalHandler) VerifyCredentials(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	user, err := h.svc.VerifyCredentials(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// GetUserByEmail resolves a user by email for other services.
func (h *InternalHandler) GetUserByEmail(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", "email query param is required"))
		return
	}

	user, err := h.svc.GetUserByEmail(c.Request.Context(), email)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// GetUserByID resolves a user by id for other services (e.g. auth refresh).
func (h *InternalHandler) GetUserByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", "id is required"))
		return
	}

	user, err := h.svc.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}