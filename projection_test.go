package silo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterDimension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/register" {
			t.Errorf("expected path /register, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected method POST, got %s", r.Method)
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		if payload["type"] != "dimension" || payload["name"] != "score" {
			t.Errorf("invalid payload: %v", payload)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "registered"})
	}))
	defer server.Close()

	s := &Silo{
		BaseURL: server.URL,
		client:  server.Client(),
		Token:   "test_token",
	}

	err := s.RegisterDimension("score", DimensionOptions{
		BucketWidth: 100,
		Direction:   "desc",
		Scale:       1,
	})

	if err != nil {
		t.Fatalf("RegisterDimension failed: %v", err)
	}
}

func TestTopK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/top" {
			t.Errorf("expected path /top, got %s", r.URL.Path)
		}

		q := r.URL.Query()
		if q.Get("dimension") != "score" || q.Get("k") != "3" {
			t.Errorf("invalid query params: %v", q)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TopResponse{
			OK:      true,
			Results: []string{"user1", "user2", "user3"},
		})
	}))
	defer server.Close()

	s := &Silo{
		BaseURL: server.URL,
		client:  server.Client(),
		Token:   "test_token",
	}

	results, err := s.TopK("score", 3, "desc")
	if err != nil {
		t.Fatalf("TopK failed: %v", err)
	}

	if len(results) != 3 || results[0] != "user1" {
		t.Errorf("unexpected results: %v", results)
	}
}
