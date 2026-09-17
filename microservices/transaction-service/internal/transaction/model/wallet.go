package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Wallet is the shared-DB phase projection of wallet-service's wallets table.
// It exists only so transfers can read/update balances inside one DB transaction.
type Wallet struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Balance   decimal.Decimal `json:"balance"`
	Currency  string          `json:"currency"`
	Status    string          `json:"status"`
	Version   int             `json:"-"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
