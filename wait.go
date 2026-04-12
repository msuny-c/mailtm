package mailtm

import (
	"context"
	"strings"
	"time"
)

type WaitOptions struct {
	Timeout       time.Duration
	PollInterval  time.Duration
	SubjectPrefix string
	FromContains  string
}

// WaitForFirstMessage polls for messages until one matching the filter criteria is found.
// Returns the first matching message or an error if timeout is reached.
// Default timeout is 30 seconds and poll interval is 2 seconds.
func (c *Client) WaitForFirstMessage(ctx context.Context, opts WaitOptions) (*Message, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 2 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()

	check := func(m Message) bool {
		if opts.SubjectPrefix != "" && !strings.HasPrefix(m.Subject, opts.SubjectPrefix) {
			return false
		}
		if opts.FromContains != "" && !strings.Contains(m.From.Address, opts.FromContains) && !strings.Contains(m.From.Name, opts.FromContains) {
			return false
		}
		return true
	}

	for {
		coll, err := c.ListMessages(ctx, 1)
		if err != nil {
			return nil, err
		}
		for _, m := range coll.Member {
			if check(m) {
				return c.GetMessage(ctx, m.ID)
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
