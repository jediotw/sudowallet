package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Transaction struct {
	ID               string          `json:"id"`
	SenderWalletID   *string         `json:"sender_wallet_id"` // nil for credit-only transactions (e.g. top-ups)
	ReceiverWalletID string          `json:"receiver_wallet_id"`
	Amount           decimal.Decimal `json:"amount"`
	Description      string          `json:"description"`
	IdempotencyKey   string          `json:"idempotency_key"`
	Status           string          `json:"status"` // success, pending, failed
	CreatedAt        time.Time       `json:"created_at"`
}
