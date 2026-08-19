package handler

import (
	"net/http"

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
	c.JSON(http.StatusOK, gin.H{"succes": true, "data": user})
}
