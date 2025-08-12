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

func WithBaseURL(u string) Option {
	return func(c *Client) error {
		c.baseURL = trimRightSlash(u)
		return nil
	}
}

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		if hc != nil {
			c.hc = hc
		}
		return nil
	}
}

func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		c.userAgent = ua
		return nil
	}
}

func WithRetry(r RetryPolicy) Option {
	return func(c *Client) error {
		c.retry = r
		return nil
	}
}

func WithRateLimiter(rl *TokenBucket) Option {
	return func(c *Client) error {
		c.rl = rl
		return nil
	}
}

func WithLogger(l Logger) Option {
	return func(c *Client) error {
		if l != nil {
			c.logger = l
		}
		return nil
	}
}

func WithClock(cl Clock) Option {
	return func(c *Client) error {
		if cl != nil {
			c.clock = cl
		}
		return nil
	}
}
