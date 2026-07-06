package silo

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Silo is the main client for interacting with the service.
type Silo struct {
	BaseURL string
	Token   string
	wsID    string
	client  *http.Client

	mu         sync.RWMutex
	epoch      int64
	epochDelta int64
	lastSync   time.Time
	signatures map[string]string
	clockSkew  int64

	reqCounter uint64
	lctState   *State
}

// Connect establishes a connection to the specified URI.
func Connect(connectionURI string) (*Silo, error) {
	if !strings.HasPrefix(connectionURI, "silo://") {
		return nil, fmt.Errorf("invalid connection URI")
	}
	rawURI := strings.Replace(connectionURI, "silo://", "http://", 1)
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URI")
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

	var b [4]byte
	rand.Read(b[:])
	startCount := uint64(binary.BigEndian.Uint32(b[:]))

	s := &Silo{
		BaseURL: fmt.Sprintf("%s://%s", scheme, host),
		Token:   token,
		wsID:    wsID,
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
			},
		},
		reqCounter: startCount,
		epochDelta: 30,
		lctState:   NewState([]byte(token)),
	}
	err = s.Sync()
	if err != nil {
		return nil, err
	}
	return s, nil
}

type handshakeResponse struct {
	SN             string            `json:"sn"`
	Epoch          int64             `json:"epoch"`
	EpochDelta     int64             `json:"epoch_delta"`
	Signatures     map[string]string `json:"signatures"`
	ServerTime     int64             `json:"server_time"`
	Challenge      uint64            `json:"challenge,omitempty"`
	ChallengeState uint64            `json:"challenge_state,omitempty"`
}

var lastIntegrity string

// Sync synchronizes the local state with the service.
func (s *Silo) Sync() error {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()

	proof := s.GenerateProof("", "", nonce, sequence, epoch)

	req, _ := http.NewRequest("POST", s.BaseURL+"/handshake", nil)
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("X-Silo-Command", CommandSync)
	req.Header.Set("X-Silo-Priority", PriorityHigh)

	if lastIntegrity != "" {
		req.Header.Set("X-Silo-Integrity", lastIntegrity)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sync_failed_status_%d", resp.StatusCode)
	}
	var res handshakeResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	if res.Challenge != 0 {
		chalState := NewState([]byte(s.Token))
		chalState.S = new(big.Int).SetUint64(res.ChallengeState)

		v1, v2, v3 := chalState.mix(byte(res.Challenge & 0xFF))
		integrity := fmt.Sprintf("%d:%d:%d", v1, v2, v3)
		lastIntegrity = hex.EncodeToString([]byte(integrity))
	}

	skew := res.ServerTime - time.Now().UnixNano()

	s.mu.Lock()
	s.epoch = res.Epoch
	s.epochDelta = res.EpochDelta
	s.lastSync = time.Now()
	s.signatures = res.Signatures
	s.clockSkew = skew
	s.mu.Unlock()
	return nil
}

func (s *Silo) CurrentEpoch() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.epochDelta == 0 {
		return 0
	}
	now := time.Now().UnixNano() + s.clockSkew
	return (now / 1e9) / s.epochDelta
}

func (s *Silo) NextSequence() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Metropolis Rule: Sequence is monotonic + predictive
	ts := time.Now().UnixNano() + s.clockSkew
	sVal := s.lctState.S.Uint64()

	return fmt.Sprintf("%019d%016x", ts, sVal)
}

type SignInResult struct {
	APIKey      string `json:"api_key"`
	RecoveryKey string `json:"recovery_key"`
}

// SignIn authenticates the user credentials.
func SignIn(baseURL, name, password string) (*SignInResult, error) {
	payload := map[string]string{"name": name, "password": password}
	reqBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", baseURL+"/signin", bytes.NewBuffer(reqBody))
	req.Header.Set("X-Silo-Command", CommandIdentity)
	req.Header.Set("X-Silo-Priority", PriorityHigh)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status_%d: %s", resp.StatusCode, string(body))
	}

	var result SignInResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Proof generates a fresh (nonce, sequence, proof) triple for a request.
// SSGP §3.2: each call produces a unique, one-time proof.
func (s *Silo) Proof(path string, reqHash string) (nonce string, sequence string, proof string) {
	nonce = NewNonce()
	sequence = s.NextSequence()
	proof = s.GenerateProof(path, reqHash, nonce, sequence, s.CurrentEpoch())
	return
}

// Telemetry retrieves operational statistics.
func (s *Silo) Telemetry() (map[string]interface{}, error) {
	req, _ := http.NewRequest("GET", s.BaseURL+"/stats", nil)
	req.Header.Set("X-Silo-Command", CommandSync)
	req.Header.Set("X-Silo-Priority", PriorityLow)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}
