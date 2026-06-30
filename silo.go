package silo

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Silo is the client for interacting with the Silo engine.
// Brand rule: the call prefix is always silo — never db.
type Silo struct {
	BaseURL string
	Token   string
	client  *http.Client
}

// Connect initializes a new Silo client from a connection string.
// Format: silo://workspace_id:token@host
func Connect(connectionURI string) (*Silo, error) {
	if !strings.HasPrefix(connectionURI, "silo://") {
		return nil, fmt.Errorf("invalid connection URI: must start with silo://")
	}

	rawURI := strings.Replace(connectionURI, "silo://", "http://", 1)
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection URI: %w", err)
	}

	token, _ := parsed.User.Password()
	host := parsed.Host

	return &Silo{
		BaseURL: fmt.Sprintf("http://%s", host),
		Token:   token,
		client:  &http.Client{},
	}, nil
}
