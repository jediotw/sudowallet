package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WalletClient talks to wallet-service's internal API. In the shared-database
// phase the wallet row could be written directly by user-service, but we already
// enforce ownership: user-service never touches the wallets table.
type WalletClient interface {
	CreateWallet(ctx context.Context, userID string, currency string) error
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

func (c *httpWalletClient) CreateWallet(ctx context.Context, userID string, currency string) error {
	payload := map[string]string{
		"user_id":  userID,
		"currency": currency,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/wallets", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("wallet-service create wallet failed with status %d", resp.StatusCode)
	}

	return nil
}
