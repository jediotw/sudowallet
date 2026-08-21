package model

import (
	"github.com/shopspring/decimal"
	"time"
)

type LedgerEntry struct {
	ID            string          `json:"id"`
	WalletID      string          `json:"wallet_id"`
	TransactionID string          `json:"transaction_id"`
	Amount        decimal.Decimal `json:"amount"`
	EntryType     string          `json:"type"` // credit or debit
	CreatedAt     time.Time       `json:"created_at"`
}
