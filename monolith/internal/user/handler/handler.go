package handler

import (
	"net/http"
	"os"

	"path/filepath"

	"github.com/gin-gonic/gin"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/dto"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/service"
)

type UserHandler struct {
	svc service.UserService
}

func NewUserHandler(svc service.UserService) *UserHandler {
	return &UserHandler{
		svc: svc,
	}
}

// Register godoc
// @Summary      Register a new user
// @Description  Creates a user account and an associated wallet. Returns the created user.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        request  body      dto.CreateUserRequest  true  "Registration payload"
// @Success      201      {object}  model.User
// @Failure 400 {object} errors.AppError "Invalid input"
// @Failure 409 {object} errors.AppError "Email already registered"
// @Failure 500 {object} errors.AppError "Internal server error"
// @Router       /users/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		//register the error input into gin context
		//this error is an application level error
		c.Error(customErr.NewAppError(http.StatusBadRequest, "Invalid Request Input", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		// this  error occured when tried to call the register service so log this
		//register this error to the middleware
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, user)
}
func (h *UserHandler) GetProfile(c *gin.Context) {
	id := c.Param("id")
	user, err := h.svc.GetProfile(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, user)
}
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	id := c.Param("id")
	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "Invalid Request Input", err.Error()))
		return
	}
	user, err := h.svc.UpdateProfile(c.Request.Context(), id, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, user)
}
func (h *UserHandler) Login(c *gin.Context) {
	//check if user is already deleted, if yes then return error
	deletedAt, exist := c.Get("deletedAt")
	if exist && deletedAt != nil {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "USER_DELETED", "User account has been deleted."))
		return
	}
	//else proceed with login
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "Invalid Request Input", err.Error()))
		return
	}

	loginResp, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": loginResp})
}
func (h *UserHandler) GetProfileMe(c *gin.Context) {
	//get the user id from the context that was set by the auth middleware
	id := c.GetString("userID")
	//now use this id to fetch the user profile from the database
	user, err := h.svc.GetProfile(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}
	//Send this JSON as the HTTP response to the client that made this request.
	c.JSON(http.StatusOK, gin.H{"success": true, "data": user})
}

func (h *UserHandler) UpdateAvatar(c *gin.Context) {
	userID := c.GetString("userID")

	// Get file from multipart form data
	file, err := c.FormFile("avatar")
	if err != nil {
		c.Error(customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_FILE",
			"Please upload an avatar",
		))
		return
	}

	// Validate file size
	if file.Size > 5*1024*1024 {
		c.Error(customErr.NewAppError(
			http.StatusBadRequest,
			"FILE_TOO_LARGE",
			"Avatar file size should be less than 5MB",
		))
		return
	}

	// Validate MIME type
	contentType := file.Header.Get("Content-Type")

	var ext string

	switch contentType {
	case "image/jpeg":
		ext = ".jpeg"
	case "image/png":
		ext = ".png"
	case "image/gif":
		ext = ".gif"
	default:
		c.Error(customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_FILE_TYPE",
			"Only JPEG, PNG, and GIF files are allowed",
		))
		return
	}

	// Create upload directory
	uploadDir := "./uploads"
	//use can use 0777 or 07775 or os.ModePerm
	//difference between mkdirAll and mkdir is that mkdirall will create all the parent directories if they do not exist, whereas mkdir will return an error if the parent directory does not exist. So we use mkdirall here to create the uploads directory if it does not exist.
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.Error(customErr.ErrInternalServer)
		return
	}

	// Generate filename
	filename := userID + ext

	// Destination
	destination := filepath.Join(uploadDir, filename)

	// Save file ONCE
	if err := c.SaveUploadedFile(file, destination); err != nil {
		c.Error(customErr.ErrInternalServer)
		return
	}

	// Update user avatar
	avatarURL := "/uploads/" + filename

	if err := h.svc.UpdateAvatar(
		c.Request.Context(),
		userID,
		avatarURL,
	); err != nil {
		c.Error(customErr.ErrInternalServer)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Avatar updated successfully",
		"avatar_url": avatarURL,
	})
}

func (h *UserHandler) DeleteAccount(c *gin.Context) {
	id, exist := c.Get("userID")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User not authorized"))
		return
	}
	idStr, ok := id.(string)
	if !ok {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User not authorized"))
		return
	}
	err := h.svc.SoftDelete(c.Request.Context(), idStr)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User deleted successfully"})
}

func (h *UserHandler) Logout(c *gin.Context) {
	//get the token string from the context that was set by the auth middleware
	tokenString, exist := c.Get("token_string")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Token not found in context"))
		return
	}
	tokenStr, ok := tokenString.(string)
	if !ok {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token context"))
		return
	}
	err := h.svc.Logout(c.Request.Context(), tokenStr)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User logged out successfully"})
}
