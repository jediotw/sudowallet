package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/dto"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/model"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/repository"
)

type UserService interface {
	Register(ctx context.Context, req dto.CreateUserRequest) (*model.User, error)
	GetProfile(ctx context.Context, id string) (*model.User, error)
	UpdateProfile(ctx context.Context, id string, req dto.UpdateUserRequest) (*model.User, error)
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
		return nil, errors.New("email already registered")
	}
	//2. create new user
	user := &model.User{
		ID:           uuid.New().String(),
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: req.Password, //TODO: hasing the password
	}
	//3.store to the db
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	// Fetch the newly created user from the database to return the stored record.
	return s.userRepo.GetById(ctx, user.ID)
}
func (s *userService) GetProfile(ctx context.Context, id string) (*model.User, error) {
	u, err := s.userRepo.GetById(ctx, id)

	if err != nil {
		return nil, err
	}

	return u, nil
}
func (s *userService) UpdateProfile(ctx context.Context, id string, req dto.UpdateUserRequest) (*model.User, error) {
	user, err := s.userRepo.GetById(ctx, id)
	if err != nil {
		return nil, err
	}

	user.FullName = req.FullName

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return s.userRepo.GetById(ctx, id)
}
