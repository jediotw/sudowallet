package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/saurabhkr78/sudowallet/microservices/auth-service/internal/auth/model"
)

// userResponse mirrors user-service's internal JSON shape.
type userResponse struct {
	Success bool        `json:"success"`
	User    model.User  `json:"user"`
}

// UserClient talks to user-service's internal API. user-service owns the users
// table and credential verification; auth-service never reads it directly.
type UserClient interface {
	VerifyCredentials(ctx context.Context, email, password string) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
}

type httpUserClient struct {
	baseURL string
	client  *http.Client
}

func NewUserClient(baseURL string) UserClient {
	return &httpUserClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *httpUserClient) VerifyCredentials(ctx context.Context, email, password string) (*model.User, error) {
	payload := map[string]string{"email": email, "password": password}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/users/verify-credentials", bytes.NewReader(body))
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
		return nil, fmt.Errorf("user-service verify credentials failed with status %d", resp.StatusCode)
	}

	var out userResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out.User, nil
}

func (c *httpUserClient) GetByID(ctx context.Context, id string) (*model.User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/users/"+id, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user-service get user failed with status %d", resp.StatusCode)
	}

	var out userResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out.User, nil
}