package service

import (
	"context"
	"net/http"

	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/model"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/repository"
	"golang.org/x/crypto/bcrypt"
)

// VerifyCredentials checks an email/password pair against the users table. It is
// called by auth-service on login so credentials stay owned by user-service.
func (s *UserService) VerifyCredentials(ctx context.Context, email, password string) (*model.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "wrong email or password.")
		}
		logger.Error(ctx, "verify credentials: user lookup failed", "email", email, "error", err)
		return nil, customErr.ErrInternalServer
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "wrong email or password.")
	}

	return user, nil
}

// GetUserByEmail returns the active user for the given email. Called by
// auth-service (verify-credentials fallback is preferred) and transaction-service
// to resolve a receiver.
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return nil, customErr.ErrNotFound
		}
		logger.Error(ctx, "internal: user lookup by email failed", "email", email, "error", err)
		return nil, customErr.ErrInternalServer
	}
	return user, nil
}

// GetUserByID returns the active user with the given id. Called by auth-service
// when refreshing a token.
func (s *UserService) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return nil, customErr.ErrNotFound
		}
		logger.Error(ctx, "internal: user lookup by id failed", "id", id, "error", err)
		return nil, customErr.ErrInternalServer
	}
	return user, nil
}