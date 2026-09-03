package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	auth "github.com/saurabhkr78/sudowallet/monolith/internal/auth"
	email "github.com/saurabhkr78/sudowallet/monolith/internal/email"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/dto"
	userModel "github.com/saurabhkr78/sudowallet/monolith/internal/user/model"
	userRepo "github.com/saurabhkr78/sudowallet/monolith/internal/user/repository"
	otpGenerator "github.com/saurabhkr78/sudowallet/monolith/internal/utils"
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
	RequestPasswordReset(ctx context.Context, email string) error

	VerifyPasswordReset(
		ctx context.Context,
		email string,
		code string,
	) (resetToken string, err error)

	ResetPassword(
		ctx context.Context,
		resetToken string,
		newPassword string,
	) error
	GenerateAndSendOTP(ctx context.Context, userID string, emailAddr string, otpType string) error
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

	// Secret used to HMAC OTPs before storing them in Redis.
	// This prevents someone who obtains Redis data from
	// directly brute-forcing the 6-digit OTP offline.
	otpSecret []byte
}

func NewUserService(
	db *sql.DB,
	uRepo userRepo.UserRepository,
	wRepo walletRepo.WalletRepository,
	rdb *redis.Client,
	emailSender email.EmailSender,
	otpSecret []byte,
) *userService {
	return &userService{
		db:          db,
		userRepo:    uRepo,
		walletRepo:  wRepo,
		rdb:         rdb,
		emailSender: emailSender,
		otpSecret:   otpSecret,
	}
}

const (
	emailVerificationOTP = "email_verification"
	passwordResetOTP     = "password_reset"

	otpTTL         = 5 * time.Minute
	resetTokenTTL  = 10 * time.Minute
	maxOTPAttempts = 5
)

func otpRedisKey(otpType, userID string) string {
	return fmt.Sprintf("otp:%s:%s", otpType, userID)
}

func resetTokenRedisKey(tokenHash string) string {
	return fmt.Sprintf("password_reset_token:%s", tokenHash)
}
func (s *userService) hashOTP(code string) string {
	mac := hmac.New(sha256.New, s.otpSecret)
	mac.Write([]byte(code))

	return hex.EncodeToString(mac.Sum(nil))
}

var verifyAndConsumeOTPScript = redis.NewScript(`
	local stored = redis.call("GET", KEYS[1])

	if not stored then
		return 0
	end

	if stored ~= ARGV[1] then
		return -1
	end

	redis.call("DEL", KEYS[1])

	return 1
`)

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
	// generate and send otp
	if err := s.GenerateAndSendOTP(ctx, user.ID, user.Email, "email_verification"); err != nil {
		logger.Log.Error("failed to generate and send otp during registration", "error", err)
	}
	return s.userRepo.GetById(ctx, user.ID)

}
func (s *userService) GenerateAndSendOTP(
	ctx context.Context,
	userID string,
	emailAddr string,
	otpType string,
) error {

	otpCode, err := otpGenerator.GenerateOTP(6)
	if err != nil {
		logger.Log.Error("failed to generate otp", "error", err)
		return customErr.ErrInternalServer
	}

	key := otpRedisKey(otpType, userID)

	// We do not store the plaintext OTP in Redis.
	//
	// Since the OTP contains only 6 digits, storing the plaintext
	// would allow someone with access to Redis to immediately
	// obtain the OTP.
	//
	// Instead, store an HMAC of the OTP.
	otpHash := s.hashOTP(otpCode)

	err = s.rdb.Set(
		ctx,
		key,
		otpHash,
		otpTTL,
	).Err()

	if err != nil {
		logger.Log.Error(
			"failed to store otp in redis",
			"error", err,
		)
		return customErr.ErrInternalServer
	}

	// Send the email asynchronously so that the API request
	// does not have to wait for the email provider.
	go func() {
		emailCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		var subject, body string

		switch otpType {
		case emailVerificationOTP:
			subject = "SudoWallet - Verify Your Email"
			body = fmt.Sprintf(
				"Your verification code is %s\n\nThis code will expire in 5 minutes.\n\nThank you!",
				otpCode,
			)

		case passwordResetOTP:
			subject = "SudoWallet - Reset Your Password"
			body = fmt.Sprintf(
				"Your password reset code is %s\n\nThis code will expire in 5 minutes.\n\nThank you!",
				otpCode,
			)

		default:
			subject = "SudoWallet - Security Code"
			body = fmt.Sprintf(
				"Your code is %s\n\nThis code will expire in 5 minutes.\n\nThank you!",
				otpCode,
			)
		}

		if err := s.emailSender.SendEmail(
			emailCtx,
			emailAddr,
			subject,
			body,
		); err != nil {
			logger.Log.Error(
				"failed to send email",
				"error", err,
			)
		}
	}()

	return nil
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

	// Email verification changes two pieces of state:
	//
	// 1. OTP in Redis → consumed
	// 2. User in PostgreSQL → email_verified = true
	//
	// Redis and PostgreSQL cannot participate in the same
	// database transaction.
	//
	// Therefore, OTP verification + OTP consumption is made
	// atomic inside Redis, and then we update the user in
	// PostgreSQL.

	key := otpRedisKey(
		emailVerificationOTP,
		userID,
	)

	otpHash := s.hashOTP(req.Code)

	result, err := verifyAndConsumeOTPScript.Run(
		ctx,
		s.rdb,
		[]string{key},
		otpHash,
	).Int()

	if err != nil {
		logger.Log.Error(
			"failed to verify otp in redis",
			"error", err,
		)
		return customErr.ErrInternalServer
	}

	switch result {
	case 0:
		return customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_OTP",
			"Invalid or expired OTP.",
		)

	case -1:
		return customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_OTP",
			"Invalid or expired OTP.",
		)

	case 1:
		// OTP was valid and has already been consumed atomically.
	default:
		logger.Log.Error(
			"unexpected otp verification result",
			"result", result,
		)
		return customErr.ErrInternalServer
	}

	// OTP has been successfully verified and consumed.
	//
	// Now update the permanent user state in PostgreSQL.
	if err := s.userRepo.UpdateVerificationStatus(
		ctx,
		userID,
		true,
	); err != nil {
		logger.Log.Error(
			"failed to update email verification status",
			"error", err,
		)
		return customErr.ErrInternalServer
	}

	return nil
}

func (s *userService) RequestPasswordReset(
	ctx context.Context,
	email string,
) error {

	// Password reset must not reveal whether an email
	// belongs to an existing account.
	//
	// An attacker should receive the same external response
	// for:
	//
	// 1. Existing email
	// 2. Non-existing email
	//
	// The handler therefore always returns a generic response.

	user, err := s.userRepo.GetByEmail(ctx, email)

	if err != nil {

		// A missing user is not a server error from the
		// perspective of the password-reset flow.
		//
		// If the repository uses sql.ErrNoRows to represent
		// "not found", treat it exactly like user == nil.
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}

		// A real database/infrastructure failure should still
		// be logged and returned internally.
		logger.Log.Error(
			"failed to lookup user for password reset",
			"error", err,
		)

		return customErr.ErrInternalServer
	}

	if user == nil {
		// User does not exist.
		//
		// Deliberately return nil so the caller cannot
		// distinguish this case from an existing account.
		return nil
	}

	// Generate OTP and store it in Redis.
	//
	// The key is:
	//
	// otp:password_reset:{userID}
	//
	// The OTP automatically expires after 5 minutes.
	return s.GenerateAndSendOTP(
		ctx,
		user.ID,
		user.Email,
		passwordResetOTP,
	)
}

func (s *userService) VerifyPasswordReset(
	ctx context.Context,
	email string,
	code string,
) (string, error) {

	// We first resolve the user internally.
	//
	// The user identity is never returned to the client.
	user, err := s.userRepo.GetByEmail(ctx, email)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", customErr.NewAppError(
				http.StatusBadRequest,
				"INVALID_OTP",
				"Invalid or expired OTP.",
			)
		}

		logger.Log.Error(
			"failed to lookup user during password reset verification",
			"error", err,
		)

		return "", customErr.ErrInternalServer
	}

	if user == nil {
		return "", customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_OTP",
			"Invalid or expired OTP.",
		)
	}

	key := otpRedisKey(
		passwordResetOTP,
		user.ID,
	)

	otpHash := s.hashOTP(code)

	// Verify and consume the OTP atomically.
	//
	// This prevents two concurrent requests from successfully
	// using the same OTP.
	result, err := verifyAndConsumeOTPScript.Run(
		ctx,
		s.rdb,
		[]string{key},
		otpHash,
	).Int()

	if err != nil {
		logger.Log.Error(
			"failed to verify password reset otp",
			"error", err,
		)

		return "", customErr.ErrInternalServer
	}

	switch result {
	case 0, -1:
		return "", customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_OTP",
			"Invalid or expired OTP.",
		)

	case 1:
		// OTP successfully verified and consumed.

	default:
		logger.Log.Error(
			"unexpected password reset otp result",
			"result", result,
		)

		return "", customErr.ErrInternalServer
	}

	// The OTP has now served its purpose.
	//
	// We generate a new high-entropy reset token.
	// The OTP itself must NOT be used as the password-reset
	// authorization credential.
	resetTokenBytes := make([]byte, 32)

	if _, err := rand.Read(resetTokenBytes); err != nil {
		logger.Log.Error(
			"failed to generate password reset token",
			"error", err,
		)
		return "", customErr.ErrInternalServer
	}

	resetToken := hex.EncodeToString(resetTokenBytes)

	// The reset token is high entropy, so SHA-256 is sufficient
	// for storing its digest.
	tokenHash := sha256.Sum256([]byte(resetToken))
	tokenHashString := hex.EncodeToString(tokenHash[:])

	tokenKey := resetTokenRedisKey(tokenHashString)

	err = s.rdb.Set(
		ctx,
		tokenKey,
		user.ID,
		resetTokenTTL,
	).Err()

	if err != nil {
		logger.Log.Error(
			"failed to store password reset token",
			"error", err,
		)
		return "", customErr.ErrInternalServer
	}

	return resetToken, nil
}
func (s *userService) ResetPassword(
	ctx context.Context,
	resetToken string,
	newPassword string,
) error {

	// The reset token is the temporary authorization
	// granted after successful OTP verification.
	//
	// We never trust the email supplied by the client here.
	// The user ID comes from the server-side reset token.

	tokenHash := sha256.Sum256([]byte(resetToken))
	tokenHashString := hex.EncodeToString(tokenHash[:])

	key := resetTokenRedisKey(tokenHashString)

	userID, err := s.rdb.Get(
		ctx,
		key,
	).Result()

	if err == redis.Nil {
		return customErr.NewAppError(
			http.StatusUnauthorized,
			"INVALID_RESET_TOKEN",
			"Invalid or expired password reset token.",
		)
	}

	if err != nil {
		logger.Log.Error(
			"failed to get password reset token",
			"error", err,
		)
		return customErr.ErrInternalServer
	}

	// Hash the new password before storing it.
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(newPassword),
		bcrypt.DefaultCost,
	)
	if err != nil {
		logger.Log.Error(
			"failed to hash password",
			"error", err,
		)
		return customErr.ErrInternalServer
	}

	// Update permanent user state in PostgreSQL.
	if err := s.userRepo.UpdatePassword(
		ctx,
		userID,
		string(passwordHash),
	); err != nil {
		logger.Log.Error(
			"failed to update password",
			"error", err,
		)
		return customErr.ErrInternalServer
	}

	// Password reset token is single-use.
	if err := s.rdb.Del(ctx, key).Err(); err != nil {
		logger.Log.Error(
			"failed to consume password reset token",
			"error", err,
		)
		return customErr.ErrInternalServer
	}

	return nil
}
