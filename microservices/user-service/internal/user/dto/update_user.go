package dto

type UpdateUserRequest struct {
	FullName string `json:"full_name" binding:"required"`
}
