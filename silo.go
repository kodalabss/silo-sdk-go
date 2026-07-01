package silo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Silo is the client for interacting with the Silo engine.
type Silo struct {
	BaseURL string
	Token   string
	client  *http.Client

	// Ephemeral state (Phase 5 - Stateless Evaluator)
	// These are stored in volatile RAM only.
	mu         sync.RWMutex
	sn         string // The temporary session secret from the server
	epoch      int64
	epochDelta int64
	lastSync   time.Time
}

// Connect initializes a new Silo client from a connection string.
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

	scheme := "https"
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		scheme = "http"
	}

	s := &Silo{
		BaseURL: fmt.Sprintf("%s://%s", scheme, host),
		Token:   token,
		client:  &http.Client{},
	}

	// Handshake immediately to get the first Sn
	if err := s.Handshake(); err != nil {
		// We don't fail connect, but the first request might fail if not synced
	}

	return s, nil
}

type handshakeResponse struct {
	SN         string `json:"sn"`
	Epoch      int64  `json:"epoch"`
	EpochDelta int64  `json:"epoch_delta"`
}

// Handshake fetches the temporary session secret from the server.
// This hides the evolution math from the SDK.
func (s *Silo) Handshake() error {
	req, _ := http.NewRequest("POST", s.BaseURL+"/handshake", nil)
	req.Header.Set("Authorization", "Bearer "+s.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("handshake_failed: %d", resp.StatusCode)
	}

	var res handshakeResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	s.mu.Lock()
	s.sn = res.SN
	s.epoch = res.Epoch
	s.epochDelta = res.EpochDelta
	s.lastSync = time.Now()
	s.mu.Unlock()

	return nil
}

// CurrentEpoch returns the estimated current server epoch.
func (s *Silo) CurrentEpoch() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.epochDelta == 0 {
		return 0
	}

	elapsed := int64(time.Since(s.lastSync).Seconds())
	return s.epoch + (elapsed / s.epochDelta)
}
