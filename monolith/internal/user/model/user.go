package model

import (
	"time"
)

type User struct {
	ID           string     `json:"id"`
	FullName     string     `json:"full_name"`
	Email        string     `json:"email"`
	Role         string     `json:"role"` // user, admin
	PasswordHash string     `json:"-"`    // don't expose hash password to json
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	IsVerified   bool       `json:"is_verified"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}
