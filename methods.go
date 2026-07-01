package silo

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type GetResponse struct {
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value,omitempty"`
	T     uint64          `json:"T,omitempty"`
	Error string          `json:"error,omitempty"`
}

type SetOptions struct {
	ExpectedT  uint64
	TTLSeconds int64
}

type SetResponse struct {
	OK    bool   `json:"ok"`
	T     uint64 `json:"T,omitempty"`
	Error string `json:"error,omitempty"`
}

type BatchWrite struct {
	Path      string      `json:"path"`
	Value     interface{} `json:"value"`
	ExpectedT uint64      `json:"expected_T,omitempty"`
	Proof     string      `json:"proof,omitempty"`
}

type BatchResult struct {
	OK    bool   `json:"ok"`
	T     uint64 `json:"T,omitempty"`
	Error string `json:"error,omitempty"`
}

type WatchEvent struct {
	Value json.RawMessage `json:"value"`
	T     uint64          `json:"T"`
}

func NewNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Silo) Get(path string) (json.RawMessage, uint64, error) {
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

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var result GetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, err
	}

	if !result.OK {
		return nil, 0, MapErrorCode(result.Error)
	}

	return result.Value, result.T, nil
}

func (s *Silo) GetWithPathAndProof(path, proof, nonce string) (json.RawMessage, uint64, error) {
	reqObj := map[string]string{"path": path}
	reqBody, _ := json.Marshal(reqObj)
	req, _ := http.NewRequest("GET", s.BaseURL+"/get", bytes.NewBuffer(reqBody))
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var result GetResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Value, result.T, nil
}

func (s *Silo) Set(path string, value interface{}, opts ...SetOptions) (uint64, error) {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()

	// Substance Layer: LCT Transformation
	valBytes, _ := json.Marshal(value)

	s.mu.RLock()
	sn := s.sn
	s.mu.RUnlock()

	var finalValue interface{}
	if sn != "" {
		// Data vibrates into noise before transport
		finalValue = LCTEncode(valBytes, []byte(sn))
	} else {
		finalValue = value
	}

	payload := map[string]interface{}{
		"path":  path,
		"value": finalValue,
	}

	if len(opts) > 0 {
		if opts[0].ExpectedT > 0 {
			payload["expected_T"] = opts[0].ExpectedT
		}
		if opts[0].TTLSeconds > 0 {
			payload["ttl_seconds"] = opts[0].TTLSeconds
		}
	}

	reqBody, _ := json.Marshal(payload)
	reqHash := HashBody(reqBody)
	proof := s.GenerateProof(path, reqHash, nonce, epoch)

	req, _ := http.NewRequest("PUT", s.BaseURL+"/set", bytes.NewBuffer(reqBody))
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result SetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if !result.OK {
		return 0, MapErrorCode(result.Error)
	}

	return result.T, nil
}

func (s *Silo) Del(path string) error {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()

	reqObj := map[string]string{"path": path}
	reqBody, _ := json.Marshal(reqObj)
	reqHash := HashBody(reqBody)
	proof := s.GenerateProof(path, reqHash, nonce, epoch)

	req, _ := http.NewRequest("DELETE", s.BaseURL+"/del", bytes.NewBuffer(reqBody))
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if !result.OK {
		return MapErrorCode(result.Error)
	}

	return nil
}

func (s *Silo) Batch(writes []BatchWrite) ([]BatchResult, error) {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()

	reqBody, _ := json.Marshal(map[string]interface{}{"writes": writes})
	reqHash := HashBody(reqBody)

	for i := range writes {
		writes[i].Proof = s.GenerateProof(writes[i].Path, reqHash, nonce, epoch)
	}

	finalBody, _ := json.Marshal(map[string]interface{}{"writes": writes})

	req, _ := http.NewRequest("PUT", s.BaseURL+"/batch", bytes.NewBuffer(finalBody))
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK      bool          `json:"ok"`
		Results []BatchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (s *Silo) Watch(path string) (<-chan WatchEvent, io.Closer, error) {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	proof := s.GenerateProof(path, "", nonce, epoch)

	u, _ := url.Parse(s.BaseURL + "/watch")
	q := u.Query()
	q.Set("path", path)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	events := make(chan WatchEvent)

	go func() {
		defer close(events)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			rawJSON := strings.TrimPrefix(line, "data: ")
			var event WatchEvent
			if err := json.Unmarshal([]byte(rawJSON), &event); err != nil {
				continue
			}
			events <- event
		}
	}()

	return events, resp.Body, nil
}

func (s *Silo) Register(segment string, fields []string) (map[string]string, error) {
	payload := map[string]interface{}{
		"segment": segment,
		"fields":  fields,
	}
	reqBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", s.BaseURL+"/register", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Status     string            `json:"status"`
		Normalized map[string]string `json:"normalized"`
		Error      string            `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error != "" {
		return nil, MapErrorCode(result.Error)
	}

	return result.Normalized, nil
}

type SignInResult struct {
	APIKey      string `json:"api_key"`
	RecoveryKey string `json:"recovery_key"`
}

func SignIn(baseURL, name, password string) (*SignInResult, error) {
	payload := map[string]string{"name": name, "password": password}
	reqBody, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL+"/signin", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SignInResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

type CreateWorkspaceResult struct {
	WorkspaceID      string `json:"workspace_id"`
	WorkspaceKey     string `json:"workspace_key"`
	ConnectionString string `json:"connection_string"`
}

func CreateWorkspace(baseURL, appName string) (*CreateWorkspaceResult, error) {
	payload := map[string]string{"app_name": appName}
	reqBody, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL+"/workspace/create", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result CreateWorkspaceResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
