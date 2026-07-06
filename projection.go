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

type AnchorOptions struct {
	BucketWidth int64  `json:"bucket_width"`
	Direction   string `json:"direction"`
	Scale       int    `json:"scale"`
}

type TopResponse struct {
	OK      bool     `json:"ok"`
	Results []string `json:"results"`
	Code    string   `json:"code,omitempty"`
	Coord   uint64   `json:"coord,omitempty"`
}

// Anchor configures a mathematical layer for a field.
func (s *Silo) Anchor(name string, opts AnchorOptions) error {
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
	req.Header.Set("X-Silo-Command", CommandStructure)
	req.Header.Set("X-Silo-Priority", PriorityHigh)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Status string `json:"status"`
		Code   string `json:"code,omitempty"`
		Coord  uint64 `json:"coord,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Code != "" {
		return MapErrorCode(result.Code, result.Coord)
	}

	return nil
}

// Top retrieves the highest or lowest entities in a dimension.
func (s *Silo) Top(dimension string, k int, direction string) ([]string, error) {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()

	u, _ := url.Parse(s.BaseURL + "/top")
	q := u.Query()
	q.Set("dimension", dimension)
	q.Set("k", strconv.Itoa(k))
	q.Set("direction", direction)
	u.RawQuery = q.Encode()

	projPath := "__proj__/" + dimension
	h := Resolve(s.wsID, projPath, s.signatures)
	proof := s.GenerateProof(projPath, "", nonce, sequence, epoch)

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("X-Silo-Coordinate", fmt.Sprintf("%d", h))
	req.Header.Set("X-Silo-Command", CommandJump)
	req.Header.Set("X-Silo-Priority", PriorityNormal)
	req.Header.Set("Authorization", "Bearer "+s.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result TopResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, MapErrorCode("SY_PARSE_ERROR", 0)
	}

	if !result.OK {
		return nil, MapErrorCode(result.Code, result.Coord)
	}

	return result.Results, nil
}

// Match finds entities that exactly match a categorical value.
func (s *Silo) Match(dimension string, value interface{}) ([]string, error) {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()

	u, _ := url.Parse(s.BaseURL + "/match")
	q := u.Query()
	q.Set("dimension", dimension)
	valStr := fmt.Sprintf("%v", value)
	q.Set("value", valStr)
	u.RawQuery = q.Encode()

	projPath := "__proj__/" + dimension
	h := Resolve(s.wsID, projPath, s.signatures)
	proof := s.GenerateProof(projPath, "", nonce, sequence, epoch)

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("X-Silo-Coordinate", fmt.Sprintf("%d", h))
	req.Header.Set("X-Silo-Command", CommandJump)
	req.Header.Set("X-Silo-Priority", PriorityNormal)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result TopResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, MapErrorCode(result.Code, result.Coord)
	}

	return result.Results, nil
}

// Range finds entities within a specified value range.
func (s *Silo) Range(dimension string, start, end interface{}) ([]string, error) {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()

	u, _ := url.Parse(s.BaseURL + "/range")
	q := u.Query()
	q.Set("dimension", dimension)
	q.Set("start", fmt.Sprintf("%v", start))
	q.Set("end", fmt.Sprintf("%v", end))
	u.RawQuery = q.Encode()

	projPath := "__proj__/" + dimension
	h := Resolve(s.wsID, projPath, s.signatures)
	proof := s.GenerateProof(projPath, "", nonce, sequence, epoch)

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("X-Silo-Coordinate", fmt.Sprintf("%d", h))
	req.Header.Set("X-Silo-Command", CommandJump)
	req.Header.Set("X-Silo-Priority", PriorityNormal)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result TopResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, MapErrorCode(result.Code, result.Coord)
	}

	return result.Results, nil
}

// Stats retrieves distribution metrics for a dimension.
func (s *Silo) Stats(dimension string) (map[string]interface{}, error) {
	epoch := s.CurrentEpoch()
	nonce := NewNonce()
	sequence := s.NextSequence()

	u, _ := url.Parse(s.BaseURL + "/stats")
	q := u.Query()
	q.Set("dimension", dimension)
	u.RawQuery = q.Encode()

	projPath := "__proj__/" + dimension
	h := Resolve(s.wsID, projPath, s.signatures)
	proof := s.GenerateProof(projPath, "", nonce, sequence, epoch)

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("X-Silo-Workspace-ID", s.wsID)
	req.Header.Set("X-Silo-Proof", proof)
	req.Header.Set("X-Silo-Nonce", nonce)
	req.Header.Set("X-Silo-Sequence", sequence)
	req.Header.Set("X-Silo-Coordinate", fmt.Sprintf("%d", h))
	req.Header.Set("X-Silo-Command", CommandJump)
	req.Header.Set("X-Silo-Priority", PriorityLow)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool                   `json:"ok"`
		Stats map[string]interface{} `json:"stats"`
		Code  string                 `json:"code"`
		Coord uint64                 `json:"coord"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, MapErrorCode(result.Code, result.Coord)
	}

	return result.Stats, nil
}
