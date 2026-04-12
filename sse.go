package mailtm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Event struct {
	ID    string
	Type  string
	Data  []byte
	Retry time.Duration
}

type Stream interface {
	Events() <-chan Event
	Err() error
	Close() error
}

type sseConfig struct {
	HubURL            string
	LastEventID       string
	ReconnectDelay    time.Duration
	MaxReconnectDelay time.Duration
}

type sseStream struct {
	events chan Event
	errMu  sync.Mutex
	err    error

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (s *sseStream) Events() <-chan Event { return s.events }
func (s *sseStream) Err() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}
func (s *sseStream) setErr(err error) {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	s.err = err
}
func (s *sseStream) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

type SSEOption func(*sseConfig)

// WithHubURL sets the SSE hub URL for real-time event subscriptions.
func WithHubURL(u string) SSEOption { return func(c *sseConfig) { c.HubURL = u } }

// WithLastEventID sets the last event ID to resume from when reconnecting.
func WithLastEventID(id string) SSEOption { return func(c *sseConfig) { c.LastEventID = id } }

// SubscribeAccount establishes a server-sent events (SSE) subscription for real-time account updates.
// Requires an authenticated client with a valid JWT token. Auto-reconnects on connection failures.
func (c *Client) SubscribeAccount(ctx context.Context, accountID string, opts ...SSEOption) (Stream, error) {
	cfg := sseConfig{
		HubURL:            "https://mercure.mail.tm/.well-known/mercure",
		ReconnectDelay:    500 * time.Millisecond,
		MaxReconnectDelay: 10 * time.Second,
	}
	for _, o := range opts {
		o(&cfg)
	}
	if accountID == "" {
		return nil, errors.New("accountID required")
	}
	if c.token == "" {
		return nil, errors.New("SubscribeAccount requires an authenticated client (use WithToken)")
	}

	hub, err := url.Parse(cfg.HubURL)
	if err != nil {
		return nil, err
	}
	if hub.Scheme == "" || hub.Host == "" {
		return nil, errors.New("mailtm: invalid SSE hub URL")
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &sseStream{
		events: make(chan Event, 16),
		cancel: cancel,
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(s.events)

		ck := c.clock
		lastID := cfg.LastEventID
		reconnectBase := cfg.ReconnectDelay
		backoff := cfg.ReconnectDelay
		for {
			u := *hub
			q := u.Query()
			q.Set("topic", "/accounts/"+accountID)
			if lastID != "" {
				q.Set("lastEventID", lastID)
			}
			u.RawQuery = q.Encode()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
			if err != nil {
				s.setErr(err)
				select {
				case <-ctx.Done():
					return
				case <-ck.After(backoff):
					backoff = minDur(backoff*2, cfg.MaxReconnectDelay)
					continue
				}
			}
			req.Header.Set("Accept", "text/event-stream")
			if c.token != "" {
				req.Header.Set("Authorization", "Bearer "+c.token)
			}
			resp, err := c.hc.Do(req)
			if err != nil {
				s.setErr(err)
				select {
				case <-ctx.Done():
					return
				case <-ck.After(backoff):
					backoff = minDur(backoff*2, cfg.MaxReconnectDelay)
					continue
				}
			}
			if resp.StatusCode != http.StatusOK {
				s.setErr(parseHTTPError(resp))
				select {
				case <-ctx.Done():
					return
				case <-ck.After(backoff):
					backoff = minDur(backoff*2, cfg.MaxReconnectDelay)
					continue
				}
			}
			backoff = reconnectBase

			br := bufio.NewReader(resp.Body)
			var ev Event
			var dataBuf []byte
			for {
				line, err := br.ReadString('\n')
				if err != nil {
					resp.Body.Close()
					if err == io.EOF || errors.Is(err, context.Canceled) {
						select {
						case <-ctx.Done():
							return
						default:
						}
						break
					}
					s.setErr(err)
					break
				}
				line = strings.TrimRight(line, "\r\n")
				if line == "" {
					if len(dataBuf) > 0 || ev.Type != "" || ev.ID != "" {
						ev.Data = append([]byte(nil), dataBuf...)
						select {
						case s.events <- ev:
						case <-ctx.Done():
							resp.Body.Close()
							return
						}
						if ev.ID != "" {
							lastID = ev.ID
						}
					}
					ev = Event{}
					dataBuf = dataBuf[:0]
					continue
				}
				switch {
				case strings.HasPrefix(line, "id:"):
					ev.ID = strings.TrimSpace(line[3:])
				case strings.HasPrefix(line, "event:"):
					ev.Type = strings.TrimSpace(line[6:])
				case strings.HasPrefix(line, "retry:"):
					if ms, err := strconv.ParseUint(strings.TrimSpace(line[6:]), 10, 64); err == nil {
						d := time.Duration(ms) * time.Millisecond
						ev.Retry = d
						reconnectBase = minDur(d, cfg.MaxReconnectDelay)
					}
				case strings.HasPrefix(line, "data:"):
					if len(dataBuf) > 0 {
						dataBuf = append(dataBuf, '\n')
					}
					dataBuf = append(dataBuf, strings.TrimSpace(line[5:])...)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ck.After(backoff):
				backoff = minDur(backoff*2, cfg.MaxReconnectDelay)
			}
		}
	}()
	return s, nil
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// DecodeAccountEvent decodes an SSE event containing account data into an Account struct.
func DecodeAccountEvent(e Event) (*Account, error) {
	var acc Account
	if err := json.Unmarshal(e.Data, &acc); err != nil {
		return nil, err
	}
	return &acc, nil
}
