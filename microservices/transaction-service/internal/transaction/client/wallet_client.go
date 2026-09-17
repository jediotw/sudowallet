package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	customErr "github.com/saurabhkr78/sudowallet/microservices/shared/errors"
	"github.com/shopspring/decimal"
)

// Wallet is the projection of wallet-service's wallet returned by its internal
// API. Version is the optimistic-locking value used by AdjustBalance.
type Wallet struct {
	ID       string          `json:"id"`
	UserID   string          `json:"user_id"`
	Balance  decimal.Decimal `json:"balance"`
	Currency string          `json:"currency"`
	Status   string          `json:"status"`
	Version  int64           `json:"version"`
}

type walletResponse struct {
	Success bool   `json:"success"`
	Wallet  Wallet `json:"wallet"`
	Version int64  `json:"version"`
}

type errorResponse struct {
	Error customErr.AppError `json:"error"`
}

// WalletClient talks to wallet-service's internal API. wallet-service owns the
// wallets table; transaction-service never touches it directly.
type WalletClient interface {
	ResolveByUserID(ctx context.Context, userID string) (*Wallet, error)
	AdjustBalance(ctx context.Context, walletID string, amount decimal.Decimal, expectedVersion int64) (*Wallet, error)
}

type httpWalletClient struct {
	baseURL string
	client  *http.Client
}

func NewWalletClient(baseURL string) WalletClient {
	return &httpWalletClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *httpWalletClient) ResolveByUserID(ctx context.Context, userID string) (*Wallet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/wallets/user/"+userID, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out walletResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	// wallet-service returns the version both inside the wallet and at the top
	// level (the model hides it); prefer the explicit top-level value.
	if out.Version != 0 {
		out.Wallet.Version = out.Version
	}
	return &out.Wallet, nil
}

func (c *httpWalletClient) AdjustBalance(ctx context.Context, walletID string, amount decimal.Decimal, expectedVersion int64) (*Wallet, error) {
	payload := map[string]any{
		"wallet_id":        walletID,
		"amount":           amount,
		"expected_version": expectedVersion,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/wallets/adjust", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out walletResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Version != 0 {
		out.Wallet.Version = out.Version
	}
	return &out.Wallet, nil
}

// decodeError turns wallet-service's error envelope into an AppError so callers
// can preserve status codes (404 wallet not found, 409 concurrent update).
func decodeError(resp *http.Response) error {
	var out errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err == nil && out.Error.Code != "" {
		return &out.Error
	}
	return fmt.Errorf("wallet-service returned status %d", resp.StatusCode)
}