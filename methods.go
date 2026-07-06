package silo

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
)

type GetResponse struct {
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value,omitempty"`
	T     uint64          `json:"T,omitempty"`
	Code  string          `json:"code,omitempty"`
	Coord uint64          `json:"coord,omitempty"`
}

type SetOptions struct {
	ExpectedT  uint64
	TTLSeconds int64
}

type SetResponse struct {
	OK    bool   `json:"ok"`
	T     uint64 `json:"T,omitempty"`
	Code  string `json:"code,omitempty"`
	Coord uint64 `json:"coord,omitempty"`
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
	Code  string `json:"code,omitempty"`
	Coord uint64 `json:"coord,omitempty"`
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

type GetRequest struct {
	Path string `json:"path"`
}

type SetRequest struct {
	Path       string      `json:"path"`
	Value      interface{} `json:"value"`
	ExpectedT  uint64      `json:"expected_T,omitempty"`
	TTLSeconds int64       `json:"ttl_seconds,omitempty"`
}

// Get retrieves a value from the specified path.
func (s *Silo) Get(path string) (json.RawMessage, uint64, error) {
	for retry := 0; retry < 2; retry++ {
		epoch := s.CurrentEpoch()
		nonce := NewNonce()
		sequence := s.NextSequence()

		reqObj := GetRequest{Path: path}
		reqBody, _ := json.Marshal(reqObj)
		reqHash := HashBody(reqBody)
		h := Resolve(s.wsID, path, s.signatures)
		proof := s.GenerateProof(path, reqHash, nonce, sequence, epoch)

		req, _ := http.NewRequest("GET", s.BaseURL+"/get", bytes.NewBuffer(reqBody))
		req.Header.Set("X-Silo-Workspace-ID", s.wsID)
		req.Header.Set("X-Silo-Proof", proof)
		req.Header.Set("X-Silo-Nonce", nonce)
		req.Header.Set("X-Silo-Sequence", sequence)
		req.Header.Set("X-Silo-Coordinate", fmt.Sprintf("%d", h))
		req.Header.Set("X-Silo-Command", CommandRead)
		req.Header.Set("X-Silo-Priority", PriorityNormal)
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound && retry == 0 {
				s.Sync()
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			return nil, 0, fmt.Errorf("server_error_status_%d: %s", resp.StatusCode, string(body))
		}

		var result GetResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, 0, err
		}

		if !result.OK {
			if retry == 0 && (result.Code == "ID_AUTH_FAIL" || result.Code == "GE_COORDINATE_VOID") {
				s.Sync()
				continue
			}
			return nil, 0, MapErrorCode(result.Code, result.Coord)
		}

		var hexSubstance string
		if err := json.Unmarshal(result.Value, &hexSubstance); err == nil {
			st := NewState([]byte(s.Token))
			decoded, err := SubstanceUnpack(hexSubstance, st)
			if err == nil {
				var v interface{}
				if err := msgpack.Unmarshal(decoded, &v); err == nil {
					jsonBytes, _ := json.Marshal(v)
					return jsonBytes, result.T, nil
				}
				return decoded, result.T, nil
			}
			return nil, 0, fmt.Errorf("integrity_check_failed")
		}

		return result.Value, result.T, nil
	}
	return nil, 0, fmt.Errorf("max_retries_exceeded")
}

// Set stores a value at the specified path.
func (s *Silo) Set(path string, value interface{}, opts ...SetOptions) (uint64, error) {
	for retry := 0; retry < 2; retry++ {
		valBytes, _ := msgpack.Marshal(value)

		epoch := s.CurrentEpoch()
		st := NewState([]byte(s.Token))
		finalValue := SubstancePack(valBytes, st)

		sequence := s.NextSequence()

		reqObj := SetRequest{
			Path:  path,
			Value: finalValue,
		}

		if len(opts) > 0 {
			reqObj.ExpectedT = opts[0].ExpectedT
			reqObj.TTLSeconds = opts[0].TTLSeconds
		}

		nonce := NewNonce()

		reqBody, _ := json.Marshal(reqObj)
		reqHash := HashBody(reqBody)
		h := Resolve(s.wsID, path, s.signatures)
		proof := s.GenerateProof(path, reqHash, nonce, sequence, epoch)

		req, _ := http.NewRequest("PUT", s.BaseURL+"/set", bytes.NewBuffer(reqBody))
		req.Header.Set("X-Silo-Workspace-ID", s.wsID)
		req.Header.Set("X-Silo-Proof", proof)
		req.Header.Set("X-Silo-Nonce", nonce)
		req.Header.Set("X-Silo-Sequence", sequence)
		req.Header.Set("X-Silo-Coordinate", fmt.Sprintf("%d", h))
		req.Header.Set("X-Silo-Command", CommandWrite)
		req.Header.Set("X-Silo-Priority", PriorityHigh)
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound && retry == 0 {
				s.Sync()
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			return 0, fmt.Errorf("server_error_status_%d: %s", resp.StatusCode, string(body))
		}

		var result SetResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return 0, err
		}

		if !result.OK {
			if retry == 0 && (result.Code == "ID_AUTH_FAIL" || result.Code == "GE_COORDINATE_VOID") {
				s.Sync()
				continue
			}
			return 0, MapErrorCode(result.Code, result.Coord)
		}

		return result.T, nil
	}
	return 0, fmt.Errorf("max_retries_exceeded")
}

// Del removes the value at the specified path.
func (s *Silo) Del(path string) error {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()

	reqObj := GetRequest{Path: path}
	reqBody, _ := json.Marshal(reqObj)
	reqHash := HashBody(reqBody)
	proof := s.GenerateProof(path, reqHash, nonce, sequence, epoch)

	req, _ := http.NewRequest("DELETE", s.BaseURL+"/del", bytes.NewBuffer(reqBody))
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("X-Silo-Command", CommandWrite)
	req.Header.Set("X-Silo-Priority", PriorityHigh)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool   `json:"ok"`
		Code  string `json:"code,omitempty"`
		Coord uint64 `json:"coord,omitempty"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if !result.OK {
		return MapErrorCode(result.Code, result.Coord)
	}

	return nil
}

// Batch performs multiple write operations in a single request.
func (s *Silo) Batch(writes []BatchWrite) ([]BatchResult, error) {
	epoch := s.CurrentEpoch()

	for i := range writes {
		valBytes, _ := msgpack.Marshal(writes[i].Value)
		st := NewState([]byte(s.Token))
		writes[i].Value = SubstancePack(valBytes, st)
	}

	nonce := NewNonce()
	sequence := s.NextSequence()

	payload := map[string]interface{}{"writes": writes}
	finalBody, _ := json.Marshal(payload)
	reqHash := HashBody(finalBody)

	proof := s.GenerateProof("", reqHash, nonce, sequence, epoch)

	req, _ := http.NewRequest("PUT", s.BaseURL+"/batch", bytes.NewBuffer(finalBody))
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("X-Silo-Command", CommandWrite)
	req.Header.Set("X-Silo-Priority", PriorityHigh)
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

// Watch subscribes to real-time updates for a path.
func (s *Silo) Watch(path string) (<-chan WatchEvent, io.Closer, error) {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()
	proof := s.GenerateProof(path, "", nonce, sequence, epoch)

	u, _ := url.Parse(s.BaseURL + "/watch")
	q := u.Query()
	q.Set("path", path)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("X-Silo-Command", CommandObserve)
	req.Header.Set("X-Silo-Priority", PriorityNormal)

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

// Schema defines the structure for a data segment.
func (s *Silo) Schema(segment string, fields []string) (map[string]string, error) {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()

	payload := map[string]interface{}{
		"segment": segment,
		"fields":  fields,
	}
	reqBody, _ := StableMarshal(payload)
	reqHash := HashBody(reqBody)
	proof := s.GenerateProof("", reqHash, nonce, sequence, epoch)

	req, _ := http.NewRequest("POST", s.BaseURL+"/register", bytes.NewBuffer(reqBody))
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("X-Silo-Command", CommandStructure)
	req.Header.Set("X-Silo-Priority", PriorityHigh)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Status     string            `json:"status"`
		Normalized map[string]string `json:"normalized"`
		Code       string            `json:"code,omitempty"`
		Coord      uint64            `json:"coord,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != "" {
		return nil, MapErrorCode(result.Code, result.Coord)
	}

	return result.Normalized, nil
}

// Trace retrieves historical state changes for a path.
func (s *Silo) Trace(path string) (<-chan WatchEvent, io.Closer, error) {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()
	proof := s.GenerateProof(path, "", nonce, sequence, epoch)

	u, _ := url.Parse(s.BaseURL + "/trace")
	q := u.Query()
	q.Set("path", path)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("X-Silo-Command", CommandObserve)
	req.Header.Set("X-Silo-Priority", PriorityLow)

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

type ProvisionResult struct {
	WorkspaceID      string `json:"workspace_id"`
	WorkspaceKey     string `json:"workspace_key"`
	ConnectionString string `json:"connection_string"`
}

// Provision creates a new workspace.
func Provision(baseURL, appName string) (*ProvisionResult, error) {
	payload := map[string]string{"app_name": appName}
	reqBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/workspace/create", bytes.NewBuffer(reqBody))
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

	var result ProvisionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Ping checks the health of the connection.
func (s *Silo) Ping() error {
	req, _ := http.NewRequest("GET", s.BaseURL+"/health", nil)
	req.Header.Set("X-Silo-Command", CommandSync)
	req.Header.Set("X-Silo-Priority", PriorityLow)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service_unavailable")
	}
	return nil
}
