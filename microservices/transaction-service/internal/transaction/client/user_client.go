package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"
)

// ErrUserNotFound is returned when user-service has no user for an email.
var ErrUserNotFound = errors.New("user not found")

// userResponse mirrors user-service's internal JSON shape.
type userResponse struct {
	Success bool `json:"success"`
	User    struct {
		ID string `json:"id"`
	} `json:"user"`
}

// UserClient talks to user-service's internal API. user-service owns the users
// table; transaction-service only needs the receiver's id.
type UserClient interface {
	GetIDByEmail(ctx context.Context, email string) (string, error)
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

func (c *httpUserClient) GetIDByEmail(ctx context.Context, email string) (string, error) {
	endpoint := c.baseURL + "/internal/users/by-email?email=" + url.QueryEscape(email)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrUserNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("user-service lookup failed")
	}

	var out userResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.User.ID, nil
}