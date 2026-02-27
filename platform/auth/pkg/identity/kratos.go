// Package identity provides Ory Kratos integration for user authentication.
package identity

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// KratosClient communicates with Ory Kratos for session validation.
type KratosClient struct {
	publicURL string
	client    *http.Client
}

// Session represents a validated Kratos session.
type Session struct {
	ID         string    `json:"id"`
	Active     bool      `json:"active"`
	IdentityID string    `json:"-"`
	Email      string    `json:"-"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// kratosSession is the raw Kratos API response shape.
type kratosSession struct {
	ID       string `json:"id"`
	Active   bool   `json:"active"`
	Identity struct {
		ID     string `json:"id"`
		Traits struct {
			Email string `json:"email"`
		} `json:"traits"`
	} `json:"identity"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewKratosClient creates a client for the Kratos public API.
func NewKratosClient(publicURL string) *KratosClient {
	return &KratosClient{
		publicURL: publicURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ValidateSession checks a session cookie or token against Kratos.
// Pass the raw Cookie header value or a session token.
func (k *KratosClient) ValidateSession(cookie string, token string) (*Session, error) {
	req, err := http.NewRequest("GET", k.publicURL+"/sessions/whoami", nil)
	if err != nil {
		return nil, fmt.Errorf("kratos: build request: %w", err)
	}

	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if token != "" {
		req.Header.Set("X-Session-Token", token)
	}

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kratos: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("kratos: session invalid or expired")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kratos: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var ks kratosSession
	if err := json.NewDecoder(resp.Body).Decode(&ks); err != nil {
		return nil, fmt.Errorf("kratos: decode: %w", err)
	}

	if !ks.Active {
		return nil, fmt.Errorf("kratos: session not active")
	}

	return &Session{
		ID:         ks.ID,
		Active:     ks.Active,
		IdentityID: ks.Identity.ID,
		Email:      ks.Identity.Traits.Email,
		ExpiresAt:  ks.ExpiresAt,
	}, nil
}
