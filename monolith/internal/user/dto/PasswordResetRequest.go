package dto

type PasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}
