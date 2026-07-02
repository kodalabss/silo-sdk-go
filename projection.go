package silo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type DimensionOptions struct {
	BucketWidth int64  `json:"bucket_width"`
	Direction   string `json:"direction"`
	Scale       int    `json:"scale"`
}

func (s *Silo) RegisterDimension(name string, opts DimensionOptions) error {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()

	payload := map[string]interface{}{
		"type":         "dimension",
		"name":         name,
		"bucket_width": opts.BucketWidth,
		"direction":    opts.Direction,
		"scale":        opts.Scale,
	}
	reqBody, _ := json.Marshal(payload)
	reqHash := HashBody(reqBody)
	proof := s.GenerateProof("", reqHash, nonce, sequence, epoch)

	req, _ := http.NewRequest("POST", s.BaseURL+"/register", bytes.NewBuffer(reqBody))
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Error != "" {
		return MapErrorCode(result.Error)
	}

	return nil
}

type TopResponse struct {
	OK      bool     `json:"ok"`
	Results []string `json:"results"`
	Error   string   `json:"error,omitempty"`
}

func (s *Silo) TopK(dimension string, k int, direction string) ([]string, error) {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()

	u, _ := url.Parse(s.BaseURL + "/top")
	q := u.Query()
	q.Set("dimension", dimension)
	q.Set("k", strconv.Itoa(k))
	q.Set("direction", direction)
	u.RawQuery = q.Encode()

	// Projections live in the __proj__ namespace
	projPath := "__proj__/" + dimension
	proof := s.GenerateProof(projPath, "", nonce, sequence, epoch)

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("Authorization", "Bearer "+s.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TopK_request_failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result TopResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, MapErrorCode(string(body))
	}

	if !result.OK {
		return nil, MapErrorCode(result.Error)
	}

	return result.Results, nil
}
