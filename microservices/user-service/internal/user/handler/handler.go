package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/dto"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/service"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) Register(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error()))
		return
	}

	userID, err := h.svc.Register(c.Request.Context(), &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Registration successful. Check your email for the verification code.",
		"data":    gin.H{"user_id": userID},
	})
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := currentUserID(c)
	if userID == "" {
		return
	}

	user, err := h.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    mapUser(user),
	})
}

func (h *UserHandler) GetProfileByID(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_USER_ID", "user id is required"))
		return
	}

	user, err := h.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    mapUser(user),
	})
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_USER_ID", "user id is required"))
		return
	}

	// Only allow updating your own profile.
	if authID := currentUserID(c); authID != "" && authID != userID {
		c.Error(customErr.NewAppError(http.StatusForbidden, "FORBIDDEN", "You can only update your own profile"))
		return
	}

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error()))
		return
	}

	user, err := h.svc.UpdateProfile(c.Request.Context(), userID, &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Profile updated successfully",
		"data":    mapUser(user),
	})
}

func (h *UserHandler) UpdateAvatar(c *gin.Context) {
	userID := currentUserID(c)
	if userID == "" {
		return
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "AVATAR_REQUIRED", "avatar file is required"))
		return
	}

	user, err := h.svc.UpdateAvatar(c.Request.Context(), userID, file)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Avatar updated successfully",
		"data":    mapUser(user),
	})
}

func (h *UserHandler) SoftDelete(c *gin.Context) {
	userID := currentUserID(c)
	if userID == "" {
		return
	}

	if err := h.svc.SoftDelete(c.Request.Context(), userID); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Account deleted successfully",
	})
}

func (h *UserHandler) VerifyEmail(c *gin.Context) {
	userID := currentUserID(c)
	if userID == "" {
		return
	}

	var req dto.VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error()))
		return
	}

	if err := h.svc.VerifyEmail(c.Request.Context(), userID, &req); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Email verified successfully",
	})
}

func (h *UserHandler) RequestPasswordReset(c *gin.Context) {
	var req dto.PasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error()))
		return
	}

	if err := h.svc.RequestPasswordReset(c.Request.Context(), &req); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "If the email exists, a reset code has been sent.",
	})
}

func (h *UserHandler) VerifyPasswordReset(c *gin.Context) {
	var req dto.VerifyPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error()))
		return
	}

	token, err := h.svc.VerifyPasswordReset(c.Request.Context(), &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Code verified. Use the reset token to set a new password.",
		"data":    gin.H{"reset_token": token},
	})
}

func (h *UserHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error()))
		return
	}

	if err := h.svc.ResetPassword(c.Request.Context(), &req); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password reset successfully",
	})
}

// currentUserID reads the userID claim injected by AuthMiddleware.
func currentUserID(c *gin.Context) string {
	v, exist := c.Get("userID")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated"))
		return ""
	}
	id, ok := v.(string)
	if !ok || id == "" {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		return ""
	}
	return id
}

func mapUser(u interface{}) gin.H {
	return gin.H{"user": u}
}
