package service

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	auth "github.com/saurabhkr78/sudowallet/monolith/internal/auth"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/dto"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/model"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/repository"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	Register(ctx context.Context, req dto.CreateUserRequest) (*model.User, error)
	GetProfile(ctx context.Context, id string) (*model.User, error)
	UpdateProfile(ctx context.Context, id string, req dto.UpdateUserRequest) (*model.User, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error)
}
type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(uRepo repository.UserRepository) *userService {
	return &userService{userRepo: uRepo}
}

func (s *userService) Register(ctx context.Context, req dto.CreateUserRequest) (*model.User, error) {
	//check if email id is already registered in db
	existing, _ := s.userRepo.GetByEmail(ctx, req.Email)
	if existing != nil {
		//return a custom error why not return a internal server error? because this is a known error and we want to expose it to the client
		return nil, customErr.NewAppError(http.StatusConflict, "EMAIL_ALREADY_REGISTERED", "Email is already registered.")
	}
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	//2. create new user
	user := &model.User{
		ID:           uuid.New().String(),
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: string(hashedBytes),
	}
	//3.store to the db
	if err := s.userRepo.Create(ctx, user); err != nil {
		//return a internal server error if user creation fails but why not custom error? because this is an unexpected error and we dont want to expose the internal server error to the client
		return nil, customErr.NewAppError(http.StatusInternalServerError, "USER_CREATION_FAILED", "Failed to create user.")
	}
	// Fetch the newly created user from the database to return the stored record.
	return s.userRepo.GetById(ctx, user.ID)
}
func (s *userService) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	//find user by email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.")
	}
	//verify the hashed password with the provided password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		//create a new AppError with this as there is no custom error for this
		return nil, customErr.NewAppError(http.StatusUnauthorized, "Invalid Credentials", "Wrong email or password")
	}
	//generate jwt tokens
	accessToken, err := auth.GenerateJWT(user.ID, user.Email, time.Hour*1)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	refreshToken, err := auth.GenerateJWT(user.ID, user.Email, time.Hour*24*7)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	//return the tokens in the Loginresponse format
	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
func (s *userService) GetProfile(ctx context.Context, id string) (*model.User, error) {
	u, err := s.userRepo.GetById(ctx, id)

	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "User not found.")
	}

	return u, nil
}
func (s *userService) UpdateProfile(ctx context.Context, id string, req dto.UpdateUserRequest) (*model.User, error) {
	user, err := s.userRepo.GetById(ctx, id)
	if err != nil {
		//custom error is used here because this is a known error and we want to expose it to the client
		return nil, customErr.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "User not found.")
	}

	user.FullName = req.FullName
	//why internal server error? because this is an unexpected error and we dont want to expose the internal server error to the client?
	//how this is an internal server error? proof?

	if err := s.userRepo.Update(ctx, user); err != nil {
		//why update failed? so internal server error!!
		return nil, customErr.ErrInternalServer
		//return nil, customErr.NewAppError(http.StatusInternalServerError, "USER_UPDATE_FAILED", "Failed to update user.")
	}

	return s.userRepo.GetById(ctx, id)
}
