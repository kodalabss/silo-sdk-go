package sdk

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/kodalabs/silo/internal/api"
	"github.com/kodalabs/silo/internal/chunk"
	"github.com/kodalabs/silo/internal/read"
	"github.com/kodalabs/silo/internal/watch"
	"github.com/kodalabs/silo/internal/write"
	"github.com/kodalabs/silo/ring"
)

func setupTestServer() (*httptest.Server, *write.Pipeline) {
	bucket1 := chunk.NewMemBucket("bucket1")
	buckets := map[string]ring.Bucket{"bucket1": bucket1}
	r := ring.NewRing([]string{"bucket1"})
	tickets := write.NewTicketCounter()
	watchRegistry := watch.NewRegistry()

	writer := &write.Pipeline{
		Ring:    r,
		Buckets: buckets,
		Tickets: tickets,
		Watch:   watchRegistry,
	}
	reader := &read.Pipeline{
		Ring:    r,
		Buckets: buckets,
	}

	router := api.NewRouter(writer, reader)
	server := httptest.NewServer(router)
	return server, writer
}

func TestSDK_RoundTrip(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	// Use a dummy connection string that points to our test server
	connStr := fmt.Sprintf("silo://ws:koda_wk_test@%s", server.URL[7:])
	client, err := Connect(connStr)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	path := "users/u_1/name"
	expectedVal := "Alice"

	// 1. client.Set()
	t1, err := client.Set(path, expectedVal)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if t1 != 1 {
		t.Errorf("Expected T=1, got %d", t1)
	}

	// 2. client.Get()
	raw, t2, err := client.Get(path)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if t2 != 1 {
		t.Errorf("Expected T=1 in Get, got %d", t2)
	}

	var val string
	json.Unmarshal(raw, &val)
	if val != expectedVal {
		t.Errorf("Value mismatch: got %s, want %s", val, expectedVal)
	}
}

func TestSDK_ConditionalWrite(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	client, _ := Connect(fmt.Sprintf("silo://ws:koda_wk_test@%s", server.URL[7:]))
	path := "balance"

	// Initial set
	t1, _ := client.Set(path, 100)

	// Success path
	t2, err := client.Set(path, 110, SetOptions{ExpectedT: t1})
	if err != nil {
		t.Errorf("Set with matching ExpectedT failed: %v", err)
	}
	if t2 != 2 {
		t.Errorf("Expected T=2, got %d", t2)
	}

	// Conflict path
	_, err = client.Set(path, 120, SetOptions{ExpectedT: t1})
	if err != ErrVersionConflict {
		t.Errorf("Expected ErrVersionConflict, got %v", err)
	}
}

func TestSDK_Batch(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	client, _ := Connect(fmt.Sprintf("silo://ws:koda_wk_test@%s", server.URL[7:]))

	// Pre-populate for conflict
	client.Set("p2", "v0")

	results, err := client.Batch([]BatchWrite{
		{Path: "p1", Value: "v1"},
		{Path: "p2", Value: "v2", ExpectedT: 999}, // Should fail
	})

	if err != nil {
		t.Fatalf("Batch failed: %v", err)
	}

	if !results[0].OK {
		t.Errorf("Batch[0] should be OK")
	}
	if results[1].OK || results[1].Error != "version_conflict" {
		t.Errorf("Batch[1] should have failed with version_conflict, got %v", results[1].Error)
	}
}

func TestSDK_Watch(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	client, _ := Connect(fmt.Sprintf("silo://ws:koda_wk_test@%s", server.URL[7:]))
	path := "live"

	events, closer, err := client.Watch(path)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	expectedVal := "msg_1"
	go func() {
		defer wg.Done()
		time.Sleep(200 * time.Millisecond)
		client.Set(path, expectedVal)
	}()

	done := make(chan bool)
	go func() {
		select {
		case event, ok := <-events:
			if !ok {
				t.Errorf("Channel closed unexpectedly")
			} else {
				var val string
				if err := json.Unmarshal(event.Value, &val); err != nil {
					t.Errorf("Failed to unmarshal value: %v", err)
				}
				if val != expectedVal {
					t.Errorf("Watch received wrong value: %s", val)
				}
			}
		case <-time.After(5 * time.Second):
			t.Errorf("Watch timed out waiting for event")
		}
		done <- true
	}()

	<-done

	err = closer.Close()
	if err != nil {
		t.Errorf("Closer failed: %v", err)
	}

	select {
	case _, ok := <-events:
		if ok {
			t.Errorf("Channel should have been closed")
		}
	case <-time.After(1 * time.Second):
		t.Errorf("Channel did not close within timeout")
	}

	wg.Wait()
}
