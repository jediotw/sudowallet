package dto

import (
	"github.com/shopspring/decimal"
)

type CreateWalletRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	Currency string `json:"currency" binding:"omitempty"`
}

type AdjustBalanceRequest struct {
	WalletID        string          `json:"wallet_id" binding:"required"`
	Amount          decimal.Decimal `json:"amount" binding:"required"` // positive = credit, negative = debit
	ExpectedVersion int64           `json:"expected_version" binding:"required"`
}
