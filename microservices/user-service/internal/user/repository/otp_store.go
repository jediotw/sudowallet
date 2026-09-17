package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	OTPTypeEmailVerification = "email_verification"
	OTPTypePasswordReset     = "password_reset"
)

func otpRedisKey(otpType, userID string) string {
	return fmt.Sprintf("otp:%s:%s", otpType, userID)
}

func resetTokenRedisKey(tokenHash string) string {
	return fmt.Sprintf("password_reset_token:%s", tokenHash)
}

// VerifyAndConsumeOTPScript atomically checks and consumes an OTP so two
// concurrent requests cannot both succeed with the same code.
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

// OTPStore keeps one-time codes and password-reset tokens in Redis.
type OTPStore interface {
	SaveOTP(ctx context.Context, otpType, userID, otpHash string, ttl time.Duration) error
	// VerifyAndConsumeOTP returns 1 on success, 0 when absent, -1 when mismatched.
	VerifyAndConsumeOTP(ctx context.Context, otpType, userID, otpHash string) (int, error)
	SaveResetToken(ctx context.Context, tokenHash, userID string, ttl time.Duration) error
	GetResetTokenUserID(ctx context.Context, tokenHash string) (string, error)
	DeleteResetToken(ctx context.Context, tokenHash string) error
}

type redisOTPStore struct {
	rdb *redis.Client
}

func NewOTPStore(rdb *redis.Client) OTPStore {
	return &redisOTPStore{rdb: rdb}
}

func (s *redisOTPStore) SaveOTP(ctx context.Context, otpType, userID, otpHash string, ttl time.Duration) error {
	return s.rdb.Set(ctx, otpRedisKey(otpType, userID), otpHash, ttl).Err()
}

func (s *redisOTPStore) VerifyAndConsumeOTP(ctx context.Context, otpType, userID, otpHash string) (int, error) {
	return verifyAndConsumeOTPScript.Run(
		ctx,
		s.rdb,
		[]string{otpRedisKey(otpType, userID)},
		otpHash,
	).Int()
}

func (s *redisOTPStore) SaveResetToken(ctx context.Context, tokenHash, userID string, ttl time.Duration) error {
	return s.rdb.Set(ctx, resetTokenRedisKey(tokenHash), userID, ttl).Err()
}

func (s *redisOTPStore) GetResetTokenUserID(ctx context.Context, tokenHash string) (string, error) {
	return s.rdb.Get(ctx, resetTokenRedisKey(tokenHash)).Result()
}

func (s *redisOTPStore) DeleteResetToken(ctx context.Context, tokenHash string) error {
	return s.rdb.Del(ctx, resetTokenRedisKey(tokenHash)).Err()
}
