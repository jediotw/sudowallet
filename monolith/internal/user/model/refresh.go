package model

import "time"

type RefreshToken struct {
	ID        string     `db:"id"`
	Token     string     `db:"token"`
	UserID    string     `db:"user_id"`
	ExpiresAt time.Time  `db:"expires_at"`
	Revoked   bool       `db:"revoked"`
	RevokedAt *time.Time `db:"revoked_at"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}
