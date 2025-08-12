package mailtm

import (
	"net/http"
	"time"
)

type Option func(*Client) error

type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Errorf(format string, args ...any)
}

type noopLogger struct{}

func (noopLogger) Debugf(string, ...any) {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Errorf(string, ...any) {}

type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
	After(d time.Duration) <-chan time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time                         { return time.Now() }
func (systemClock) Sleep(d time.Duration)                  { time.Sleep(d) }
func (systemClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

type RetryPolicy struct {
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Jitter      float64
	StatusCodes map[int]struct{}
}

// DefaultRetryPolicy returns a retry policy with exponential backoff for common transient errors.
// Retries up to 4 times with base delay of 200ms, max delay of 5 seconds, and 25% jitter.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 4,
		BaseBackoff: 200 * time.Millisecond,
		MaxBackoff:  5 * time.Second,
		Jitter:      0.25,
		StatusCodes: map[int]struct{}{
			http.StatusTooManyRequests:    {},
			http.StatusBadGateway:         {},
			http.StatusServiceUnavailable: {},
			http.StatusGatewayTimeout:     {},
		},
	}
}

type TokenBucket struct {
	capacity   float64
	tokens     float64
	rate       float64
	lastRefill time.Time
	clock      Clock
}

// NewTokenBucket creates a new token bucket rate limiter with the specified capacity and refill rate.
// If capacity or ratePerSecond are <= 0, defaults to 8 are used.
// If clock is nil, the system clock is used.
func NewTokenBucket(capacity int, ratePerSecond float64, clock Clock) *TokenBucket {
	if capacity <= 0 {
		capacity = 8
	}
	if ratePerSecond <= 0 {
		ratePerSecond = 8
	}
	if clock == nil {
		clock = systemClock{}
	}
	now := clock.Now()
	return &TokenBucket{
		capacity:   float64(capacity),
		tokens:     float64(capacity),
		rate:       ratePerSecond,
		lastRefill: now,
		clock:      clock,
	}
}

func (tb *TokenBucket) take(ctx Context, n float64) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		now := tb.clock.Now()
		elapsed := now.Sub(tb.lastRefill).Seconds()
		if elapsed > 0 {
			tb.tokens = min(tb.capacity, tb.tokens+elapsed*tb.rate)
			tb.lastRefill = now
		}
		if tb.tokens >= n {
			tb.tokens -= n
			return nil
		}
		needed := n - tb.tokens
		seconds := needed / tb.rate
		if seconds < 0.001 {
			seconds = 0.001
		}
		ch := tb.clock.After(time.Duration(seconds * float64(time.Second)))
		if elapsed <= 0 {
			<-ctx.Done()
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
}

// WithBaseURL sets the base URL for the API client.
func WithBaseURL(u string) Option {
	return func(c *Client) error {
		c.baseURL = trimRightSlash(u)
		return nil
	}
}

// WithHTTPClient sets a custom HTTP client for making requests.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		if hc != nil {
			c.hc = hc
		}
		return nil
	}
}

// WithUserAgent sets a custom User-Agent header for requests.
func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		c.userAgent = ua
		return nil
	}
}

// WithRetry configures the retry policy for handling transient errors.
func WithRetry(r RetryPolicy) Option {
	return func(c *Client) error {
		c.retry = r
		return nil
	}
}

// WithRateLimiter sets a custom rate limiter to control request frequency.
func WithRateLimiter(rl *TokenBucket) Option {
	return func(c *Client) error {
		c.rl = rl
		return nil
	}
}

// WithLogger sets a custom logger for debugging and monitoring.
func WithLogger(l Logger) Option {
	return func(c *Client) error {
		if l != nil {
			c.logger = l
		}
		return nil
	}
}

// WithClock sets a custom clock implementation, primarily for testing.
func WithClock(cl Clock) Option {
	return func(c *Client) error {
		if cl != nil {
			c.clock = cl
		}
		return nil
	}
}
