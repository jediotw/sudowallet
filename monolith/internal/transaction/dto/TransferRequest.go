package dto

import (
	"github.com/shopspring/decimal"
)

// TransferRequest represents the request payload for transferring funds between wallets.
// validate:"gt=0": Enforces that the field value must be strictly positive.The json.Unmarshal() function ignores the validate tag
type TransferRequest struct {
	ReceiverEmail  string          `json:"receiver_email" binding:"required,email" example:"receiver_email@example.com"`
	Amount         decimal.Decimal `json:"amount" binding:"required,gt=0" example:"100.00"`
	Description    string          `json:"description" example:"Transfer description"`
	IdempotencyKey string          `json:"idempotency_key" binding:"required" example:"idempotency_key"`
}
