package dto

type ResetPasswordRequest struct {
	ResetToken         string `json:"reset_token" binding:"required"`
	NewPassword        string `json:"new_password" binding:"required"`
	NewPasswordConfirm string `json:"new_password_confirm" binding:"required"`
}
