package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/saurabhkr78/sudowallet/microservices/auth-service/internal/auth/client"
	"github.com/saurabhkr78/sudowallet/microservices/auth-service/internal/auth/model"
	"github.com/saurabhkr78/sudowallet/microservices/auth-service/internal/auth/repository"
	sharedAuth "github.com/saurabhkr78/sudowallet/microservices/shared/auth"
	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
)

type AuthService interface {
	Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error)
	RefreshToken(ctx context.Context, oldTokenString string) (*model.LoginResponse, error)
	Logout(ctx context.Context, tokenString string) error
}

type authService struct {
	rdb        *redis.Client
	rtRepo     repository.RefreshTokenRepository
	userClient client.UserClient
}

func NewAuthService(rdb *redis.Client, rtRepo repository.RefreshTokenRepository, userClient client.UserClient) AuthService {
	return &authService{
		rdb:        rdb,
		rtRepo:     rtRepo,
		userClient: userClient,
	}
}

func (s *authService) Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error) {
	// user-service owns credentials; ask it to verify the email/password pair.
	user, err := s.userClient.VerifyCredentials(ctx, req.Email, req.Password)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "wrong email or password.")
	}

	// generate access token 15 minutes
	accessToken, err := sharedAuth.GenerateJWT(user.ID, user.Email, 15*time.Minute)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// generate refresh token 7 days
	refreshToken, err := sharedAuth.GenerateJWT(user.ID, user.Email, 7*24*time.Hour)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// save token to db
	rt := &model.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Revoked:   false,
	}
	if err := s.rtRepo.Create(ctx, rt); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// return the tokens
	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, oldTokenString string) (*model.LoginResponse, error) {
	// 1. Look up token in db
	rt, err := s.rtRepo.GetByToken(ctx, oldTokenString)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Refresh token invalid.")
	}

	// 2. Token breach detection
	if rt.Revoked {
		_ = s.rtRepo.RevokeAllByUserID(ctx, rt.UserID)
		return nil, customErr.NewAppError(http.StatusUnauthorized, "TOKEN_BREACH_DETECTED", "Token breach detected. Please login again.")
	}

	// 3. Check if token is expired
	if time.Now().After(rt.ExpiresAt) {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "EXPIRED_REFRESH_TOKEN", "Refresh token expired. Please login again.")
	}

	// 4. Revoke old token
	if err := s.rtRepo.Revoke(ctx, oldTokenString); err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 5. Get user details from user-service to generate new JWT
	user, err := s.userClient.GetByID(ctx, rt.UserID)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 6. Generate access token & new refresh token
	newAccessToken, err := sharedAuth.GenerateJWT(user.ID, user.Email, 15*time.Minute)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	newRefreshTokenString, err := sharedAuth.GenerateJWT(user.ID, user.Email, 7*24*time.Hour)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}

	// 7. Save new Refresh Token to Database
	newRT := &model.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     newRefreshTokenString,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Revoked:   false,
	}
	if err := s.rtRepo.Create(ctx, newRT); err != nil {
		return nil, customErr.ErrInternalServer
	}

	return &model.LoginResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshTokenString,
	}, nil
}

func (s *authService) Logout(ctx context.Context, tokenString string) error {
	// Validate token
	claims, err := sharedAuth.ValidateToken(tokenString)
	if err != nil {
		return customErr.NewAppError(http.StatusUnauthorized, "INVALID_TOKEN", "token is invalid or expired.")
	}

	// Revoke all refresh tokens from user that logged out
	if err := s.rtRepo.RevokeAllByUserID(ctx, claims.UserID); err != nil {
		return customErr.NewAppError(http.StatusUnauthorized, "REVOKE_FAILED", "Failed to revoke refresh token.")
	}

	// Calculate remaining active token time
	expirationTime := claims.ExpiresAt.Time
	timeLeft := time.Until(expirationTime)

	if timeLeft <= 0 {
		return nil
	}

	// Insert into redis blacklist
	blacklistKey := fmt.Sprintf("blacklist:%s", tokenString)
	err = s.rdb.Set(ctx, blacklistKey, "logged_out", timeLeft).Err()
	if err != nil {
		return customErr.ErrInternalServer
	}

	return nil
}
