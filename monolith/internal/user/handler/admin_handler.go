package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	commonDto "github.com/saurabhkr78/sudowallet/monolith/internal/common/dto"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
)

func (h *UserHandler) AdminGetUsers(c *gin.Context) {
	var params commonDto.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	users, meta, err := h.svc.GetAllUsers(c.Request.Context(), params)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, commonDto.PaginatedResponse{
		Success: true,
		Data:    users,
		Meta:    *meta,
	})
}
