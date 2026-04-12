package mailtm

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type wrappedTransport struct {
	base   http.RoundTripper
	getUA  func() string
	getTok func() string
	rl     *TokenBucket
	retry  RetryPolicy
	log    Logger
	clock  Clock
}

// unwrapMailTMTransport returns the innermost RoundTripper beneath any SDK
// wrappedTransport layers so WithToken does not stack duplicate auth headers.
func unwrapMailTMTransport(rt http.RoundTripper) http.RoundTripper {
	for {
		wt, ok := rt.(*wrappedTransport)
		if !ok || wt == nil {
			return rt
		}
		rt = wt.base
	}
}

func (t *wrappedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	method := strings.ToUpper(req.Method)

	var bodyBytes []byte
	var getBody func() (io.ReadCloser, error)
	if req.Body != nil && req.GetBody != nil {
		getBody = req.GetBody
	} else if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		bodyBytes = b
		getBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(bodyBytes)), nil }
	}

	attempts := t.retry.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	var lastResp *http.Response
	var lastErr error
	var backoff time.Duration

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctxDone(req.Context()); err != nil {
			return nil, err
		}
		if t.rl != nil {
			if err := t.rl.take(req.Context(), 1); err != nil {
				return nil, err
			}
		}
		r2 := req.Clone(req.Context())
		if getBody != nil {
			rc, err := getBody()
			if err != nil {
				return nil, err
			}
			r2.Body = rc
		}
		if ua := t.getUA; ua != nil {
			if s := ua(); s != "" {
				r2.Header.Set("User-Agent", s)
			}
		}
		if tok := t.getTok; tok != nil {
			if s := tok(); s != "" {
				r2.Header.Set("Authorization", "Bearer "+s)
			}
		}
		if r2.Header.Get("Accept") == "" {
			r2.Header.Set("Accept", "application/ld+json")
		}

		resp, err := t.base.RoundTrip(r2)
		lastResp, lastErr = resp, err

		if err != nil {
			if attempt < attempts && isIdempotent(method) {
				backoff = t.nextBackoff(0)
				t.log.Debugf("mailtm: retry %s %s after error %v (attempt %d/%d, backoff %v)",
					method, req.URL.Redacted(), err, attempt, attempts, backoff)
				select {
				case <-t.clock.After(backoff):
					continue
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
			}
			return nil, err
		}

		if attempt < attempts && isIdempotent(method) && t.shouldRetry(resp.StatusCode) {
			delay := retryAfterDelay(resp.Header.Get("Retry-After"), t.clock.Now())
			if delay == 0 {
				delay = t.nextBackoff(attempt)
			}
			t.log.Debugf("mailtm: retry %s %s after status %d (attempt %d/%d, delay %v)",
				method, req.URL.Redacted(), resp.StatusCode, attempt, attempts, delay)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			select {
			case <-t.clock.After(delay):
				continue
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}

		return resp, err
	}
	return lastResp, lastErr
}

func (t *wrappedTransport) shouldRetry(code int) bool {
	_, ok := t.retry.StatusCodes[code]
	return ok
}

func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func (t *wrappedTransport) nextBackoff(attempt int) time.Duration {
	base := t.retry.BaseBackoff
	max := t.retry.MaxBackoff
	if base <= 0 {
		base = 200 * time.Millisecond
	}
	if max <= 0 {
		max = 5 * time.Second
	}
	exp := min(base*time.Duration(1<<maxInt(attempt-1, 0)), max)
	j := t.retry.Jitter
	if j <= 0 {
		j = 0.25
	}
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return exp
	}
	rnd := float64(binary.LittleEndian.Uint64(buf[:])) / float64(math.MaxUint64)
	scale := 1 + (2*j*rnd - j)
	return time.Duration(float64(exp) * scale)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ctxDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func retryAfterDelay(h string, now time.Time) time.Duration {
	if h == "" {
		return 0
	}
	if s, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && s >= 0 {
		return time.Duration(s) * time.Second
	}
	if when, err := http.ParseTime(h); err == nil && !when.IsZero() {
		if when.After(now) {
			return when.Sub(now)
		}
	}
	return 0
}
