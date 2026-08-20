package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Wallet struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Balance   decimal.Decimal `json:"balance"`
	Currency  string          `json:"currency"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Status    string          `json:"status"` //whether wallet is active or inactive
	Version   int             `json:"-"`      //for optimistic locking:Version tells us which version of the database row the client is currently working with
}
