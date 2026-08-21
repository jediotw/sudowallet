package dto

import (
	"github.com/shopspring/decimal"
)

// TopUpRequest represents the request payload for topping up a wallet.
// validate:"gt=0": Enforces that the field value must be strictly positive.
type TopUpRequest struct {
	Amount         decimal.Decimal `json:"amount" binding:"required,gt=0" example:"100.00"`
	IdempotencyKey string          `json:"idempotency_key" binding:"required" example:"idempotency_key"`
}
