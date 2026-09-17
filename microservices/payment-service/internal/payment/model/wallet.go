package model

import "github.com/shopspring/decimal"

// Wallet is a read projection of the wallets table (owned by wallet-service).
// payment-service only reads it for reconciliation.
type Wallet struct {
	ID      string          `json:"id"`
	UserID  string          `json:"user_id"`
	Balance decimal.Decimal `json:"balance"`
}
