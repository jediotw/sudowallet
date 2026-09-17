package dto

import (
	"github.com/shopspring/decimal"
)

type TransferRequest struct {
	ReceiverEmail  string          `json:"receiver_email" binding:"required,email" example:"receiver@example.com"`
	Amount         decimal.Decimal `json:"amount" binding:"required" example:"100.00"`
	Description    string          `json:"description" example:"Transfer description"`
	IdempotencyKey string          `json:"idempotency_key" binding:"required" example:"idempotency_key"`
}
