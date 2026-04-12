package mailtm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newClientWithRT(rt http.RoundTripper, opts ...Option) *Client {
	options := []Option{
		WithHTTPClient(&http.Client{Timeout: 5 * time.Second, Transport: rt}),
		WithRetry(RetryPolicy{
			MaxAttempts: 3,
			BaseBackoff: 10 * time.Millisecond,
			MaxBackoff:  50 * time.Millisecond,
			Jitter:      0,
			StatusCodes: map[int]struct{}{
				http.StatusTooManyRequests:    {},
				http.StatusBadGateway:         {},
				http.StatusServiceUnavailable: {},
				http.StatusGatewayTimeout:     {},
			},
		}),
	}
	options = append(options, opts...)
	c, _ := New(options...)
	return c
}

func mkResp(code int, body string, hdr http.Header) *http.Response {
	if hdr == nil {
		hdr = make(http.Header)
	}
	return &http.Response{StatusCode: code, Header: hdr, Body: io.NopCloser(bytes.NewBufferString(body))}
}

func TestTransport_RetriesIdempotent(t *testing.T) {
	var calls int32
	rt := rtFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Accept") == "" {
			t.Fatalf("Accept header should be set")
		}
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return mkResp(429, "rate", nil), nil
		}
		return mkResp(200, `{}`, nil), nil
	})
	c := newClientWithRT(rt)
	var out map[string]any
	if err := c.doJSON(context.Background(), http.MethodGet, "/domains", nil, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected retry, calls=%d", calls)
	}
}

func TestTransport_NoRetryOnPost(t *testing.T) {
	var calls int32
	rt := rtFunc(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return mkResp(502, "badgw", nil), nil
	})
	c := newClientWithRT(rt)
	err := c.doJSON(context.Background(), http.MethodPost, "/accounts", map[string]string{"a": "b"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("POST must not retry, calls=%d", calls)
	}
}

func TestTransport_RetryAfterRespected(t *testing.T) {
	start := time.Now()
	var calls int32
	rt := rtFunc(func(r *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			h := make(http.Header)
			h.Set("Retry-After", "1")
			return mkResp(429, "rate", h), nil
		}
		return mkResp(200, `{}`, nil), nil
	})
	c := newClientWithRT(rt)
	var out map[string]any
	if err := c.doJSON(context.Background(), http.MethodGet, "/x", nil, &out); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if time.Since(start) < 900*time.Millisecond {
		t.Fatalf("Retry-After not respected (took %s)", time.Since(start))
	}
}

func TestWithToken_ChainedUsesLatestToken(t *testing.T) {
	var saw string
	rt := rtFunc(func(r *http.Request) (*http.Response, error) {
		saw = r.Header.Get("Authorization")
		return mkResp(200, `{}`, nil), nil
	})
	c := newClientWithRT(rt)
	if err := c.WithToken("first").WithToken("second").doJSON(context.Background(), http.MethodGet, "/x", nil, nil); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if saw != "Bearer second" {
		t.Fatalf("Authorization: want %q, got %q", "Bearer second", saw)
	}
}

func TestTransport_NetworkErrorRetry(t *testing.T) {
	var calls int32
	rt := rtFunc(func(r *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return nil, errors.New("boom")
		}
		return mkResp(200, `{}`, nil), nil
	})
	c := newClientWithRT(rt)
	var out map[string]any
	if err := c.doJSON(context.Background(), http.MethodGet, "/y", nil, &out); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}
