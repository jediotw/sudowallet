package model

import (
	"time"
)

type User struct {
	ID           string     `json:"id,omitempty"`
	FullName     string     `json:"full_name,omitempty"`
	Email        string     `json:"email,omitempty"`
	PasswordHash string     `json:"-"`
	CreatedAt    time.Time  `json:"created_at,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at,omitempty"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"` //DeletedAt is often a *time.Time so nil means "not deleted".
}
