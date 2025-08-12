package mailtm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitForFirstMessage_Polling(t *testing.T) {
	var calls int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/messages" {
			n := atomic.AddInt32(&calls, 1)
			if n < 3 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"hydra:member":   []any{},
					"hydra:totalItems": 0,
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"hydra:member": []map[string]any{
					{"id": "m1", "subject": "Hello", "from": map[string]any{"name":"X","address":"x@y"}},
				},
				"hydra:totalItems": 1,
			})
			return
		}
		if r.URL.Path == "/messages/m1" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "m1", "subject": "Hello", "from": map[string]any{"name":"X","address":"x@y"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer s.Close()

	c, _ := New(WithBaseURL(s.URL))
	ctx := context.Background()
	msg, err := c.WaitForFirstMessage(ctx, WaitOptions{
		Timeout:      5 * time.Second,
		PollInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if msg.ID != "m1" || msg.Subject != "Hello" {
		t.Fatalf("bad message: %+v", msg)
	}
}
