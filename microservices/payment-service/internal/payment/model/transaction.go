package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Transaction is the read projection of the transactions table, which is owned
// by transaction-service. payment-service only reads it for the daily report.
type Transaction struct {
	ID               string          `json:"id"`
	SenderWalletID   string          `json:"sender_wallet_id,omitempty"`
	ReceiverWalletID string          `json:"receiver_wallet_id"`
	Amount           decimal.Decimal `json:"amount"`
	Status           string          `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
}
