package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/model"
	"github.com/shopspring/decimal"
)

type transactionsResponse struct {
	Success bool                `json:"success"`
	Data    []*model.Transaction `json:"data"`
}

type ledgerEntriesResponse struct {
	Success bool              `json:"success"`
	Data    []*model.LedgerEntry `json:"data"`
}

type ledgerBalanceResponse struct {
	Success bool            `json:"success"`
	Balance decimal.Decimal `json:"balance"`
}

// TransactionClient talks to transaction-service's internal API, which owns the
// transactions and ledger_entries tables.
type TransactionClient interface {
	GetTransactionsByDate(ctx context.Context, start, end time.Time) ([]*model.Transaction, error)
	GetLedgerBalance(ctx context.Context, walletID string) (decimal.Decimal, error)
	GetLedgerEntries(ctx context.Context, walletID string) ([]*model.LedgerEntry, error)
}

type httpTransactionClient struct {
	baseURL string
	client  *http.Client
}

func NewTransactionClient(baseURL string) TransactionClient {
	return &httpTransactionClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *httpTransactionClient) GetTransactionsByDate(ctx context.Context, start, end time.Time) ([]*model.Transaction, error) {
	endpoint := c.baseURL + "/internal/transactions?from=" + url.QueryEscape(start.Format(time.RFC3339)) + "&to=" + url.QueryEscape(end.Format(time.RFC3339))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("transaction-service get transactions failed with status %d", resp.StatusCode)
	}

	var out transactionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

func (c *httpTransactionClient) GetLedgerBalance(ctx context.Context, walletID string) (decimal.Decimal, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/ledger/wallets/"+walletID+"/balance", nil)
	if err != nil {
		return decimal.Zero, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return decimal.Zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decimal.Zero, fmt.Errorf("transaction-service ledger balance failed with status %d", resp.StatusCode)
	}

	var out ledgerBalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return decimal.Zero, err
	}
	return out.Balance, nil
}

func (c *httpTransactionClient) GetLedgerEntries(ctx context.Context, walletID string) ([]*model.LedgerEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/ledger/wallets/"+walletID+"/entries", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("transaction-service ledger entries failed with status %d", resp.StatusCode)
	}

	var out ledgerEntriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Data, nil
}