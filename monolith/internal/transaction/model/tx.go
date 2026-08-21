package model

import (
	"github.com/shopspring/decimal"
)

// Transaction represents a financial transaction between wallets.
type Transaction struct {
	ID               string          `json:"id"`
	SenderWalletID   *string         `json:"sender_wallet_id"` //* string because it can be null for credit transactions
	ReceiverWalletID string          `json:"receiver_wallet_id"`
	Amount           decimal.Decimal `json:"amount"`
	Description      string          `json:"description"`
	IdempotencyKey   string          `json:"idempotency_key"`
	Status           string          `json:"status"` // success, pending, failed
	CreatedAt        string          `json:"created_at"`
}
