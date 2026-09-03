package service

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	auth "github.com/saurabhkr78/sudowallet/monolith/internal/auth"
	email "github.com/saurabhkr78/sudowallet/monolith/internal/email"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	otpGenerator "github.com/saurabhkr78/sudowallet/monolith/internal/otp/generator"
	otpModel "github.com/saurabhkr78/sudowallet/monolith/internal/otp/model"
	otpRepository "github.com/saurabhkr78/sudowallet/monolith/internal/otp/repository"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/dto"
	userModel "github.com/saurabhkr78/sudowallet/monolith/internal/user/model"
	userRepo "github.com/saurabhkr78/sudowallet/monolith/internal/user/repository"
	walletModel "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/model"
	walletRepo "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/repository"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"log"
	"net/http"
	"time"
)

type UserService interface {
	Register(ctx context.Context, req dto.CreateUserRequest) (*userModel.User, error)
	GetProfile(ctx context.Context, id string) (*userModel.User, error)
	UpdateProfile(ctx context.Context, id string, req dto.UpdateUserRequest) (*userModel.User, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error)
	VerifyEmail(ctx context.Context, userID string, req dto.VerifyEmailRequest) error
	UpdateAvatar(ctx context.Context, id string, avatarURL string) error
	SoftDelete(ctx context.Context, id string) error
	Logout(ctx context.Context, tokenString string) error
}

// if user service is dependent upon user repository and wallet repository then we can use the interface of user repository and wallet repository in the user service and like wise we call is dependency composition. This is called implicit interface implementation. The user service does not need to know the concrete implementation of the user repository and wallet repository, it just needs to know the interface. This allows us to easily swap out the implementation of the user repository and wallet repository without changing the user service. This is a good practice in software design as it promotes loose coupling and high cohesion.
type userService struct {
	userRepo    userRepo.UserRepository
	db          *sql.DB
	walletRepo  walletRepo.WalletRepository
	rdb         *redis.Client
	emailSender email.EmailSender
	otpRepo     otpRepository.OTPRepository
}

func NewUserService(db *sql.DB, uRepo userRepo.UserRepository, wRepo walletRepo.WalletRepository, rdb *redis.Client, emailSender email.EmailSender, otpRepo otpRepository.OTPRepository) *userService {
	return &userService{db: db, userRepo: uRepo, walletRepo: wRepo, rdb: rdb, emailSender: emailSender, otpRepo: otpRepo}
}

// since the responsibiity of this register function is to create a user and a wallet for that user, we can use the transaction to ensure that both the user and the wallet are created successfully or none of them are created. This is called atomicity. If the user creation fails, the wallet creation will not be attempted and vice versa. This ensures that the database remains in a consistent state.
func (s *userService) Register(ctx context.Context, req dto.CreateUserRequest) (*userModel.User, error) {
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
	user := &userModel.User{
		ID:           uuid.New().String(),
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: string(hashedBytes),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	//craete the wallet model
	wallet := &walletModel.Wallet{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Balance:   decimal.Zero,
		Currency:  "₹",
		Status:    "ACTIVE",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	//create a transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, customErr.ErrInternalServer
	}
	//now rollback the transaction if any error occurs in the following code
	defer tx.Rollback()
	//create the user in the database using the transaction

	err = s.userRepo.CreateTx(ctx, user, tx)

	if err != nil {
		return nil, customErr.NewAppError(http.StatusInternalServerError, "user creation failed", "failed to create user")
	}

	//create the wallet in the database using the transaction
	err = s.walletRepo.CreateTx(ctx, wallet, tx)
	log.Println("error:", err)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusInternalServerError, "wallet creation failed", "failed to create wallet")
	}
	//commit the transaction
	err = tx.Commit()
	if err != nil {
		return nil, customErr.NewAppError(http.StatusInternalServerError, "transaction commit failed", "failed to commit transaction")
	}

	//after commititing the transaction i need to send the email with otp for registration verification.
	otpCode, err := otpGenerator.GenerateOTP(6)
	if err != nil {
		logger.Log.Error("failed to generate otp", "error", err)
		return nil, customErr.NewAppError(http.StatusInternalServerError, "otp generation failed", "failed to generate otp")
	}
	//in order to save this otp in the db we need to take a db model
	otpModel := otpModel.OTP{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Code:      otpCode,
		Type:      "email_verification",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		CreatedAt: time.Now(),
		Used:      false,
	}
	err = s.otpRepo.Create(ctx, &otpModel)
	if err != nil {
		logger.Log.Error("failed to create otp in db", "error", err)
		return nil, customErr.NewAppError(http.StatusInternalServerError, "otp creation failed", "failed to create otp")
	}

	//ek go routine mein email send karenge taki user ko wait na karna pade aur user ko jaldi response mile. Isse user experience improve hoga.
	go func() {
		//give a new context with timeout of 10 seconds for sending the email so that if the email sending takes more than 10 seconds then it will be cancelled and we will log the error but we will not return any error to the user as the user has already been created successfully and we have already sent the response to the user. This is a good practice to avoid blocking the main thread and to improve the performance of the application.
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		subject := "Sudowallet - Verify your email"
		body := fmt.Sprintf("Hello %s,\n\nYour OTP for email verification is: %s\n\nThis OTP will expire in 5 minutes.\n\nThank you for registering with Sudowallet!", user.FullName, otpCode)
		err := s.emailSender.SendEmail(bgCtx, user.Email, subject, body)
		if err != nil {
			logger.Log.Error("failed to send email", "error", err)
		}
	}()
	//user is alredy fetched and in memory so retuirn the user
	return user, nil
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
func (s *userService) GetProfile(ctx context.Context, id string) (*userModel.User, error) {
	u, err := s.userRepo.GetById(ctx, id)

	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "User not found.")
	}

	return u, nil
}
func (s *userService) UpdateProfile(ctx context.Context, id string, req dto.UpdateUserRequest) (*userModel.User, error) {
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
func (s *userService) UpdateAvatar(ctx context.Context, id string, avatarURL string) error {
	// Check if user exists
	_, err := s.userRepo.GetById(ctx, id)
	if err != nil {
		return customErr.NewAppError(
			http.StatusNotFound,
			"USER_NOT_FOUND",
			"User not found.",
		)
	}
	if err := s.userRepo.UpdateAvatar(ctx, id, avatarURL); err != nil {
		return customErr.ErrInternalServer
	}
	return nil
}
func (s *userService) SoftDelete(ctx context.Context, id string) error {
	// Check if user exists
	user, err := s.userRepo.GetById(ctx, id)
	if err != nil {
		return customErr.NewAppError(
			http.StatusNotFound,
			"USER_NOT_FOUND",
			"User not found.",
		)
	}

	// Check if user is already deleted
	if user.DeletedAt != nil {
		return customErr.NewAppError(
			http.StatusNotFound,
			"USER_NOT_FOUND",
			"User is already deleted.",
		)
	}

	// Soft delete the user
	if err := s.userRepo.SoftDelete(ctx, user.ID); err != nil {
		return customErr.ErrInternalServer
	}

	return nil
}

// we had stored the token string in the context in the auth middleware, so we can get it from the context and blacklist it in redis when the user logs out. This is to ensure that the user cannot use the same token to access the protected routes after logging out. This is a good practice to prevent unauthorized access to the protected routes after logging out.

func (s *userService) Logout(ctx context.Context, tokenString string) error {
	//validate the token string and extract the claims
	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		return customErr.NewAppError( //match the error with the one in auth middleware validation function error
			http.StatusUnauthorized,
			"Invalid Token",
			"Token validation failed",
		)
	}
	//if there is some time left in the token expiry then calculate that since we are blocking this token till the remaining time coz user is logged out
	//get the expiry time of the token from the claims and calculate the remaining time
	expiryTime := claims.ExpiresAt.Time
	timLeft2Expire := time.Until(expiryTime)
	if timLeft2Expire <= 0 {
		return nil // Token is already expired, no need to blacklist
	}

	// now insert into redis Blacklist the token in Redis
	blacklistKey := fmt.Sprintf("blacklist:%s", tokenString)
	err = s.rdb.Set(ctx, blacklistKey, "Logged Out", timLeft2Expire).Err()
	if err != nil {
		return customErr.ErrInternalServer
	}
	return nil
}
func (s *userService) VerifyEmail(
	ctx context.Context,
	userID string,
	req dto.VerifyEmailRequest,
) error {

	// Email verification changes multiple pieces of state:
	//
	// 1. User → email_verified = true
	// 2. OTP  → used = true
	//
	// Both changes must succeed together.
	//
	// If either operation fails, we rollback the entire
	// transaction so the database remains consistent.

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return customErr.ErrInternalServer
	}

	// If anything fails before Commit(), rollback the transaction.
	defer tx.Rollback()

	// Get the active OTP inside the transaction.
	//
	// FOR UPDATE locks the OTP row until this transaction
	// commits or rolls back.
	activeOTP, err := s.otpRepo.GetActiveOTPTx(
		ctx,
		tx,
		userID,
		req.Code,
		"email_verification",
	)
	if err != nil {
		return customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_OTP",
			"Invalid or expired OTP.",
		)
	}

	// Mark the user's email as verified.
	//
	// IMPORTANT:
	// We use the SAME transaction.
	if err := s.userRepo.UpdateVerificationStatusTx(
		ctx,
		tx,
		userID,
		true,
	); err != nil {
		return customErr.ErrInternalServer
	}

	// Mark the OTP as used.
	//
	// Again, use the SAME transaction.
	if err := s.otpRepo.MarkOTPAsUsedTx(
		ctx,
		tx,
		activeOTP.ID,
	); err != nil {
		return customErr.ErrInternalServer
	}

	// Both operations succeeded.
	// Make all changes permanent.
	if err := tx.Commit(); err != nil {
		return customErr.ErrInternalServer
	}

	return nil
}
