package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/client"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/email"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/dto"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/model"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/repository"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/utils"
	"golang.org/x/crypto/bcrypt"
)

const (
	otpLength   = 6
	otpTTL      = 10 * time.Minute
	resetTTL    = 10 * time.Minute
	maxAvatarKB = 1024
	defCurrency = "" // empty -> wallet-service applies its default currency
)

var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

type UserService struct {
	userRepo     repository.UserRepository
	otpStore     repository.OTPStore
	emailSender  email.EmailSender
	walletClient client.WalletClient
	otpSecret    []byte
	uploadDir    string
}

func NewUserService(
	userRepo repository.UserRepository,
	otpStore repository.OTPStore,
	emailSender email.EmailSender,
	walletClient client.WalletClient,
	otpSecret []byte,
	uploadDir string,
) *UserService {
	return &UserService{
		userRepo:     userRepo,
		otpStore:     otpStore,
		emailSender:  emailSender,
		walletClient: walletClient,
		otpSecret:    otpSecret,
		uploadDir:    uploadDir,
	}
}

// Register creates the user, provisions their wallet via wallet-service and
// sends a verification OTP. If wallet provisioning fails the user is
// soft-deleted (compensation) so the email stays available for retry.
func (s *UserService) Register(ctx context.Context, req *dto.CreateUserRequest) (string, error) {
	if _, err := s.userRepo.GetByEmail(ctx, req.Email); err == nil {
		return "", customErr.NewAppError(409, "EMAIL_ALREADY_REGISTERED", "Email is already registered")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error(ctx, "failed to hash password", "error", err)
		return "", customErr.ErrInternalServer
	}

	user := &model.User{
		ID:           uuid.NewString(),
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: string(hashed),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		logger.Error(ctx, "failed to create user", "error", err)
		return "", customErr.ErrInternalServer
	}

	if err := s.walletClient.CreateWallet(ctx, user.ID, defCurrency); err != nil {
		logger.Error(ctx, "failed to provision wallet, compensating", "user_id", user.ID, "error", err)
		if compErr := s.userRepo.SoftDelete(ctx, user.ID); compErr != nil {
			logger.Error(ctx, "compensation soft-delete failed", "user_id", user.ID, "error", compErr)
		}
		return "", customErr.NewAppError(503, "WALLET_PROVISION_FAILED", "Registration failed, please try again")
	}

	if err := s.GenerateAndSendOTP(ctx, req.Email); err != nil {
		return "", err
	}

	return user.ID, nil
}

func (s *UserService) GenerateAndSendOTP(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return customErr.ErrNotFound
		}
		return customErr.ErrInternalServer
	}

	otp, err := utils.GenerateOTP(otpLength)
	if err != nil {
		logger.Error(ctx, "failed to generate OTP", "error", err)
		return customErr.ErrInternalServer
	}

	otpHash := s.hashOTP(otp)
	if err := s.otpStore.SaveOTP(ctx, repository.OTPTypeEmailVerification, user.ID, otpHash, otpTTL); err != nil {
		logger.Error(ctx, "failed to store OTP", "error", err)
		return customErr.ErrInternalServer
	}

	return s.emailSender.SendEmail(ctx, user.Email, "Your verification code", fmt.Sprintf("Your code: %s", otp))
}

func (s *UserService) VerifyEmail(ctx context.Context, userID string, req *dto.VerifyEmailRequest) error {
	otpHash := s.hashOTP(req.Code)

	result, err := s.otpStore.VerifyAndConsumeOTP(ctx, repository.OTPTypeEmailVerification, userID, otpHash)
	if err != nil {
		logger.Error(ctx, "failed to verify OTP", "error", err)
		return customErr.ErrInternalServer
	}

	switch result {
	case 1:
		if err := s.userRepo.UpdateVerificationStatus(ctx, userID, true); err != nil {
			logger.Error(ctx, "failed to update verification status", "error", err)
			return customErr.ErrInternalServer
		}
		return nil
	case 0:
		return customErr.NewAppError(400, "OTP_NOT_FOUND", "OTP expired or not found")
	default:
		return customErr.NewAppError(400, "OTP_MISMATCH", "OTP does not match")
	}
}

// RequestPasswordReset sends an OTP to the user's email.
func (s *UserService) RequestPasswordReset(ctx context.Context, req *dto.PasswordResetRequest) error {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return customErr.ErrNotFound
		}
		return customErr.ErrInternalServer
	}

	otp, err := utils.GenerateOTP(otpLength)
	if err != nil {
		logger.Error(ctx, "failed to generate OTP", "error", err)
		return customErr.ErrInternalServer
	}

	otpHash := s.hashOTP(otp)
	if err := s.otpStore.SaveOTP(ctx, repository.OTPTypePasswordReset, user.ID, otpHash, otpTTL); err != nil {
		logger.Error(ctx, "failed to store password reset OTP", "error", err)
		return customErr.ErrInternalServer
	}

	return s.emailSender.SendEmail(ctx, user.Email, "Password reset code", fmt.Sprintf("Your code: %s", otp))
}

// VerifyPasswordReset checks the OTP and issues a short-lived reset token.
func (s *UserService) VerifyPasswordReset(ctx context.Context, req *dto.VerifyPasswordResetRequest) (string, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return "", customErr.ErrNotFound
		}
		return "", customErr.ErrInternalServer
	}

	otpHash := s.hashOTP(req.Code)

	result, err := s.otpStore.VerifyAndConsumeOTP(ctx, repository.OTPTypePasswordReset, user.ID, otpHash)
	if err != nil {
		logger.Error(ctx, "failed to verify password reset OTP", "error", err)
		return "", customErr.ErrInternalServer
	}

	switch result {
	case 1:
		token := uuid.NewString()
		tokenHash := s.hashResetToken(token)
		if err := s.otpStore.SaveResetToken(ctx, tokenHash, user.ID, resetTTL); err != nil {
			logger.Error(ctx, "failed to store reset token", "error", err)
			return "", customErr.ErrInternalServer
		}
		return token, nil
	case 0:
		return "", customErr.NewAppError(400, "OTP_NOT_FOUND", "OTP expired or not found")
	default:
		return "", customErr.NewAppError(400, "OTP_MISMATCH", "OTP does not match")
	}
}

func (s *UserService) ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) error {
	if req.NewPassword != req.NewPasswordConfirm {
		return customErr.NewAppError(400, "PASSWORD_MISMATCH", "New passwords do not match")
	}
	if len(req.NewPassword) < 6 {
		return customErr.NewAppError(400, "INVALID_PASSWORD", "Password must be at least 6 characters")
	}

	tokenHash := s.hashResetToken(req.ResetToken)

	userID, err := s.otpStore.GetResetTokenUserID(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return customErr.NewAppError(400, "INVALID_RESET_TOKEN", "Reset token is invalid or expired")
		}
		logger.Error(ctx, "failed to load reset token", "error", err)
		return customErr.ErrInternalServer
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Error(ctx, "failed to hash new password", "error", err)
		return customErr.ErrInternalServer
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, string(hashed)); err != nil {
		logger.Error(ctx, "failed to update password", "error", err)
		return customErr.ErrInternalServer
	}

	if err := s.otpStore.DeleteResetToken(ctx, tokenHash); err != nil {
		logger.Error(ctx, "failed to delete reset token", "error", err)
	}

	return nil
}

func (s *UserService) GetProfile(ctx context.Context, userID string) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, customErr.ErrNotFound
		}
		return nil, customErr.ErrInternalServer
	}
	return user, nil
}

func (s *UserService) UpdateProfile(ctx context.Context, userID string, req *dto.UpdateUserRequest) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, customErr.ErrNotFound
		}
		return nil, customErr.ErrInternalServer
	}

	user.FullName = req.FullName
	if err := s.userRepo.Update(ctx, user); err != nil {
		logger.Error(ctx, "failed to update user", "user_id", userID, "error", err)
		return nil, customErr.ErrInternalServer
	}

	return user, nil
}

func (s *UserService) UpdateAvatar(ctx context.Context, userID string, fileHeader *multipart.FileHeader) (*model.User, error) {
	if fileHeader.Size > maxAvatarKB*1024 {
		return nil, customErr.NewAppError(400, "FILE_TOO_LARGE", fmt.Sprintf("Avatar must be smaller than %d KB", maxAvatarKB))
	}

	f, err := fileHeader.Open()
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	defer f.Close()

	buff := make([]byte, 512)
	if _, err := f.Read(buff); err != nil {
		return nil, customErr.ErrInternalServer
	}
	if !allowedImageTypes[http.DetectContentType(buff)] {
		return nil, customErr.NewAppError(400, "INVALID_IMAGE_TYPE", "Only JPEG, PNG, WebP and GIF images are allowed")
	}

	if err := os.MkdirAll(s.uploadDir, 0o755); err != nil {
		logger.Error(ctx, "failed to create upload dir", "error", err)
		return nil, customErr.ErrInternalServer
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	filename := userID + ext
	dst := filepath.Join(s.uploadDir, filename)

	out, err := os.Create(dst)
	if err != nil {
		logger.Error(ctx, "failed to create avatar file", "error", err)
		return nil, customErr.ErrInternalServer
	}
	defer out.Close()

	if _, err := io.Copy(out, f); err != nil {
		logger.Error(ctx, "failed to write avatar file", "error", err)
		return nil, customErr.ErrInternalServer
	}

	avatarURL := "/uploads/" + filename
	if err := s.userRepo.UpdateAvatar(ctx, userID, avatarURL); err != nil {
		logger.Error(ctx, "failed to update avatar url", "error", err)
		return nil, customErr.ErrInternalServer
	}

	return s.GetProfile(ctx, userID)
}

func (s *UserService) SoftDelete(ctx context.Context, userID string) error {
	if err := s.userRepo.SoftDelete(ctx, userID); err != nil {
		logger.Error(ctx, "failed to soft delete user", "user_id", userID, "error", err)
		return customErr.ErrInternalServer
	}
	return nil
}

// hashOTP returns an HMAC-SHA256 digest so the plain OTP is never stored.
func (s *UserService) hashOTP(otp string) string {
	mac := hmac.New(sha256.New, s.otpSecret)
	mac.Write([]byte(otp))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *UserService) hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
