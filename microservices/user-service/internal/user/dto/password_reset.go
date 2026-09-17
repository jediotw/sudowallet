package dto

type PasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type VerifyPasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

type ResetPasswordRequest struct {
	ResetToken         string `json:"reset_token" binding:"required"`
	NewPassword        string `json:"new_password" binding:"required"`
	NewPasswordConfirm string `json:"new_password_confirm" binding:"required"`
}
