package silo

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Silo struct {
	BaseURL string
	Token   string
	wsID    string
	client  *http.Client

	mu         sync.RWMutex
	sn         string
	epoch      int64
	epochDelta int64
	lastSync   time.Time
	signatures map[string]string

	reqCounter uint64
}

func Connect(connectionURI string) (*Silo, error) {
	if !strings.HasPrefix(connectionURI, "silo://") { return nil, fmt.Errorf("invalid connection URI") }
	rawURI := strings.Replace(connectionURI, "silo://", "http://", 1)
	parsed, err := url.Parse(rawURI)
	if err != nil { return nil, fmt.Errorf("failed to parse URI") }
	token, _ := parsed.User.Password()
	host := parsed.Host

	parts := strings.Split(strings.TrimPrefix(token, "koda_wk_"), "_")
	wsID := ""
	if len(parts) > 0 { wsID = parts[0] }

	scheme := "https"
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") { scheme = "http" }

	// Scale Hardening: Seed the counter with random 32-bit value to prevent ID collisions
	var b [4]byte
	rand.Read(b[:])
	startCount := uint64(binary.BigEndian.Uint32(b[:]))

	s := &Silo{
		BaseURL:    fmt.Sprintf("%s://%s", scheme, host),
		Token:      token,
		wsID:       wsID,
		client:     &http.Client{},
		reqCounter: startCount,
	}
	s.Handshake()
	return s, nil
}

func (s *Silo) Handshake() error {
	req, _ := http.NewRequest("POST", s.BaseURL+"/handshake", nil)
	req.Header.Set("Authorization", "Bearer "+s.Token)
	resp, err := s.client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return fmt.Errorf("handshake_failed") }
	var res handshakeResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return err }
	s.mu.Lock()
	s.sn = res.SN; s.epoch = res.Epoch; s.epochDelta = res.EpochDelta; s.lastSync = time.Now(); s.signatures = res.Signatures
	s.mu.Unlock()
	return nil
}

func (s *Silo) CurrentEpoch() int64 {
	s.mu.RLock(); defer s.mu.RUnlock()
	if s.epochDelta == 0 { return 0 }
	elapsed := int64(time.Since(s.lastSync).Seconds())
	return s.epoch + (elapsed / s.epochDelta)
}

// NextSequence: Nanosecond precision + Padded Atomic Counter.
func (s *Silo) NextSequence() string {
	ts := time.Now().UnixNano()
	count := atomic.AddUint64(&s.reqCounter, 1)
	// Format: [UnixNano (19 digits)][Padded Count (6 digits)]
	return fmt.Sprintf("%019d%06d", ts, count%1000000)
}

func (s *Silo) RawGet(path string) []byte {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()
	reqObj := map[string]string{"path": path}
	reqBody, _ := json.Marshal(reqObj)
	reqHash := HashBody(reqBody)
	proof := s.GenerateProof(path, reqHash, nonce, sequence, epoch)
	req, _ := http.NewRequest("GET", s.BaseURL+"/get", bytes.NewBuffer(reqBody))
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := s.client.Do(req)
	if resp == nil || resp.Body == nil { return nil }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body
}

type handshakeResponse struct {
	SN         string            `json:"sn"`
	Epoch      int64             `json:"epoch"`
	EpochDelta int64             `json:"epoch_delta"`
	Signatures map[string]string `json:"signatures"`
}
