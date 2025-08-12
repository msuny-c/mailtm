package mailtm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSSE_SubscribeAccount_ReconnectAndLastEventID(t *testing.T) {
	var conn int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatalf("missing Authorization header")
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("no flusher")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		n := atomic.AddInt32(&conn, 1)
		q := r.URL.Query()
		if n == 1 {
			if q.Get("lastEventID") != "" {
				t.Fatalf("unexpected lastEventID on first connect")
			}
			send := func(id string, used int) {
				ev := Account{ID: "acc", Used: used}
				b, _ := json.Marshal(ev)
				w.Write([]byte("id: " + id + "\n"))
				w.Write([]byte("data: "))
				w.Write(b)
				w.Write([]byte("\n\n"))
				flusher.Flush()
			}
			send("1", 1)
			send("2", 2)
			return
		} else {
			if q.Get("lastEventID") != "2" {
				t.Fatalf("expected lastEventID=2, got %q", q.Get("lastEventID"))
			}
			ev := Account{ID: "acc", Used: 3}
			b, _ := json.Marshal(ev)
			w.Write([]byte("id: 3\n"))
			w.Write([]byte("data: "))
			w.Write(b)
			w.Write([]byte("\n\n"))
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
			return
		}
	}))
	defer ts.Close()

	c, _ := New()
	c = c.WithToken("tkn")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stream, err := c.SubscribeAccount(ctx, "acc", WithHubURL(ts.URL))
	if err != nil {
		t.Fatalf("subscribe error: %v", err)
	}
	defer stream.Close()

	var ids []string
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for events")
		case ev, ok := <-stream.Events():
			if !ok {
				goto done
			}
			ids = append(ids, ev.ID)
			if len(ids) == 3 {
				goto done
			}
		}
	}
done:
	if strings.Join(ids, " ") != "1 2 3" {
		t.Fatalf("unexpected ids: %v", ids)
	}
}
