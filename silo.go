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
	epoch      int64
	epochDelta int64
	lastSync   time.Time
	signatures map[string]string
	clockSkew  int64 // Nano seconds skew

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

	var b [4]byte
	rand.Read(b[:])
	startCount := uint64(binary.BigEndian.Uint32(b[:]))

	s := &Silo{
		BaseURL:    fmt.Sprintf("%s://%s", scheme, host),
		Token:      token,
		wsID:       wsID,
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
			},
		},
		reqCounter: startCount,
		epochDelta: 30, // Default per SGES §2.1
	}
	err = s.Handshake()
	if err != nil { return nil, err }
	return s, nil
}

func (s *Silo) Handshake() error {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()

	// Evaluation Auth §1: Proving we have the secret by resolving the root geometry
	proof := s.GenerateProof("", "", nonce, sequence, epoch)

	req, _ := http.NewRequest("POST", s.BaseURL+"/handshake", nil)
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)

	resp, err := s.client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return fmt.Errorf("handshake_failed_status_%d", resp.StatusCode) }
	var res handshakeResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return err }

	// Clock Skew Calculation (§3.2 Reality Sync)
	//serverNowNano := res.ServerTime + int64(rtt.Nanoseconds()/2) // RTT omitted for simplicity in basic SDK
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
	s.mu.RLock(); defer s.mu.RUnlock()
	if s.epochDelta == 0 { return 0 }
	now := time.Now().UnixNano() + s.clockSkew
	return (now / 1e9) / s.epochDelta
}

func (s *Silo) NextSequence() string {
	s.mu.RLock()
	skew := s.clockSkew
	s.mu.RUnlock()

	ts := time.Now().UnixNano() + skew
	count := atomic.AddUint64(&s.reqCounter, 1)
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
	ServerTime int64             `json:"server_time"`
}
