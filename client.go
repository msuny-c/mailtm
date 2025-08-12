package mailtm

import (
	"context"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://api.mail.tm"
	defaultUA      = "mailtm-go/1.0 (+github.com/msuny-c/mailtm)"
)

type Context = context.Context

type Client struct {
	baseURL   string
	hc        *http.Client
	userAgent string
	retry     RetryPolicy
	rl        *TokenBucket
	logger    Logger
	clock     Clock

	token string
}

// New creates a new Client with the provided options.
// If no HTTP client is provided, a default client with 30-second timeout is used.
// If no rate limiter is provided, a default token bucket with 8 tokens capacity is used.
func New(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:   defaultBaseURL,
		hc:        nil,
		userAgent: defaultUA,
		retry:     DefaultRetryPolicy(),
		logger:    noopLogger{},
		clock:     systemClock{},
		rl:        nil,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	if c.hc == nil {
		c.hc = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				ForceAttemptHTTP2:     true,
			},
		}
	}
	if c.rl == nil {
		c.rl = NewTokenBucket(8, 8, c.clock)
	}
	baseRT := c.hc.Transport
	if baseRT == nil {
		baseRT = http.DefaultTransport
	}
	c.hc.Transport = &wrappedTransport{
		base:   baseRT,
		getUA:  func() string { return c.userAgent },
		getTok: func() string { return c.token },
		rl:     c.rl,
		retry:  c.retry,
		log:    c.logger,
		clock:  c.clock,
	}
	return c, nil
}

// WithToken returns a new Client instance with the provided JWT token.
// The new client shares the same configuration as the original but uses the specified token for authentication.
func (c *Client) WithToken(token string) *Client {
	cc := *c
	cc.token = token
	baseRT := c.hc.Transport
	if baseRT == nil {
		baseRT = http.DefaultTransport
	}
	cc.hc = &http.Client{
		Timeout: c.hc.Timeout,
		Transport: &wrappedTransport{
			base:   baseRT,
			getUA:  func() string { return cc.userAgent },
			getTok: func() string { return cc.token },
			rl:     cc.rl,
			retry:  cc.retry,
			log:    cc.logger,
			clock:  cc.clock,
		},
	}
	return &cc
}
