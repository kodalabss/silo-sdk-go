package silo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	wsID    string // Extracted from token, non-secret
	client  *http.Client

	mu         sync.RWMutex
	sn         string
	epoch      int64
	epochDelta int64
	lastSync   time.Time
	signatures map[string]string
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

	parts := strings.Split(strings.TrimPrefix(token, "koda_wk_"), "_")
	wsID := ""
	if len(parts) > 0 {
		wsID = parts[0]
	}

	scheme := "https"
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		scheme = "http"
	}

	s := &Silo{
		BaseURL: fmt.Sprintf("%s://%s", scheme, host),
		Token:   token,
		wsID:    wsID,
		client:  &http.Client{},
	}

	if err := s.Handshake(); err != nil {
		// Handshake failure
	}

	return s, nil
}

type handshakeResponse struct {
	SN         string            `json:"sn"`
	Epoch      int64             `json:"epoch"`
	EpochDelta int64             `json:"epoch_delta"`
	Signatures map[string]string `json:"signatures"`
}

// Handshake fetches temporary session parameters from the server.
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
	s.signatures = res.Signatures
	s.mu.Unlock()

	return nil
}

// CurrentEpoch returns the current estimated server window.
func (s *Silo) CurrentEpoch() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.epochDelta == 0 {
		return 0
	}

	elapsed := int64(time.Since(s.lastSync).Seconds())
	return s.epoch + (elapsed / s.epochDelta)
}

func (s *Silo) RawGet(path string) []byte {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	reqObj := map[string]string{"path": path}
	reqBody, _ := json.Marshal(reqObj)
	reqHash := HashBody(reqBody)
	proof := s.GenerateProof(path, reqHash, nonce, epoch)

	req, _ := http.NewRequest("GET", s.BaseURL+"/get", bytes.NewBuffer(reqBody))
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("Content-Type", "application/json")

	resp, _ := s.client.Do(req)
	body, _ := io.ReadAll(resp.Body)
	return body
}
