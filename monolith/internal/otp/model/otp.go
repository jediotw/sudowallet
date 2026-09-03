package model

import (
	"time"
)

type OTP struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	Code      string    `db:"code"`
	Type      string    `db:"type"` // email verification or password reset
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
	Used      bool      `db:"used"`
}
