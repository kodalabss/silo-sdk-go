package sdk

import (
	"bufio"
	"bytes"
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

func (s *Silo) Get(path string) (json.RawMessage, uint64, error) {
	reqBody, _ := json.Marshal(map[string]string{"path": path})
	req, _ := http.NewRequest("GET", s.BaseURL+"/get", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+s.Token)
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

type SetOptions struct {
	ExpectedT  uint64
	TTLSeconds int64
}

type SetResponse struct {
	OK    bool   `json:"ok"`
	T     uint64 `json:"T,omitempty"`
	Error string `json:"error,omitempty"`
}

func (s *Silo) Set(path string, value interface{}, opts ...SetOptions) (uint64, error) {
	payload := map[string]interface{}{
		"path":  path,
		"value": value,
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
	req, _ := http.NewRequest("PUT", s.BaseURL+"/set", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+s.Token)
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
	reqBody, _ := json.Marshal(map[string]string{"path": path})
	req, _ := http.NewRequest("DELETE", s.BaseURL+"/del", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+s.Token)
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

type BatchWrite struct {
	Path      string      `json:"path"`
	Value     interface{} `json:"value"`
	ExpectedT uint64      `json:"expected_T,omitempty"`
}

type BatchResult struct {
	OK    bool   `json:"ok"`
	T     uint64 `json:"T,omitempty"`
	Error string `json:"error,omitempty"`
}

func (s *Silo) Batch(writes []BatchWrite) ([]BatchResult, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{"writes": writes})
	req, _ := http.NewRequest("PUT", s.BaseURL+"/batch", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+s.Token)
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

type WatchEvent struct {
	Value json.RawMessage `json:"value"`
	T     uint64          `json:"T"`
}

func (s *Silo) Watch(path string) (<-chan WatchEvent, io.Closer, error) {
	u, _ := url.Parse(s.BaseURL + "/watch")
	q := u.Query()
	q.Set("path", path)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("Authorization", "Bearer "+s.Token)
	// Content-Type not strictly needed for GET with no body

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
