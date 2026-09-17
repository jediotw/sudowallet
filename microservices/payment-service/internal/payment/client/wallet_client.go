package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/model"
)

// walletResponse mirrors wallet-service's internal JSON shape for a single wallet.
type walletResponse struct {
	Success bool         `json:"success"`
	Wallet  model.Wallet `json:"wallet"`
}

type walletsResponse struct {
	Success bool           `json:"success"`
	Data    []*model.Wallet `json:"data"`
}

// WalletClient talks to wallet-service's internal API. wallet-service owns the
// wallets table; payment-service only reads it for reconciliation.
type WalletClient interface {
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, error)
	ListAll(ctx context.Context) ([]*model.Wallet, error)
}

type httpWalletClient struct {
	baseURL string
	client  *http.Client
}

func NewWalletClient(baseURL string) WalletClient {
	return &httpWalletClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *httpWalletClient) GetByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
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
		return nil, fmt.Errorf("wallet-service get wallet failed with status %d", resp.StatusCode)
	}

	var out walletResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out.Wallet, nil
}

func (c *httpWalletClient) ListAll(ctx context.Context) ([]*model.Wallet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/wallets", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wallet-service list wallets failed with status %d", resp.StatusCode)
	}

	var out walletsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Data, nil
}