package mailtm

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListAllDomains fetches all domains by following Hydra pagination.
func (c *Client) ListAllDomains(ctx context.Context) ([]Domain, error) {
	var out []Domain
	u := "/domains?page=1"
	for {
		var coll Collection[Domain]
		if err := c.doJSON(ctx, "GET", u, nil, &coll); err != nil {
			return nil, err
		}
		out = append(out, coll.Member...)
		if coll.View == nil || coll.View.Next == "" {
			break
		}
		u = coll.View.Next
	}
	return out, nil
}

// CreateAccountAndToken creates an account and immediately exchanges credentials for a JWT token.
func (c *Client) CreateAccountAndToken(ctx context.Context, address, password string) (*Account, *Token, error) {
	acc, err := c.CreateAccount(ctx, address, password)
	if err != nil {
		return nil, nil, err
	}
	tok, err := c.Token(ctx, address, password)
	if err != nil {
		return nil, nil, err
	}
	return acc, tok, nil
}

// RandomAccountOptions controls CreateRandomAccount.
type RandomAccountOptions struct {
	UsernamePrefix      string // optional prefix for username
	UsernameLen         int    // default 12 (not counting prefix)
	PasswordLen         int    // default 16
	OnlyActive          bool   // default true
	AllowPrivateDomains bool   // default false
}

// CreateRandomAccount picks a random domain from ListAllDomains (respecting filters),
// generates secure username/password, creates the account, and returns account+token+password.
// Use the returned password for subsequent logins; the token is already set in the returned client via WithToken if needed.
func (c *Client) CreateRandomAccount(ctx context.Context, opts RandomAccountOptions) (*Account, *Token, string, error) {
	if opts.UsernameLen <= 0 {
		opts.UsernameLen = 12
	}
	if opts.PasswordLen <= 0 {
		opts.PasswordLen = 16
	}
	doms, err := c.ListAllDomains(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	var pool []Domain
	for _, d := range doms {
		if opts.OnlyActive && !d.IsActive {
			continue
		}
		if !opts.AllowPrivateDomains && d.IsPrivate {
			continue
		}
		pool = append(pool, d)
	}
	if len(pool) == 0 {
		return nil, nil, "", errors.New("no suitable domains available")
	}
	di, err := cryptoRandInt(len(pool))
	if err != nil {
		return nil, nil, "", err
	}
	d := pool[di]

	userRand, err := randString(opts.UsernameLen, lettersDigits)
	if err != nil {
		return nil, nil, "", err
	}
	pass, err := randString(opts.PasswordLen, strongAlphabet)
	if err != nil {
		return nil, nil, "", err
	}
	local := opts.UsernamePrefix + userRand
	address := local + "@" + d.Domain

	acc, tok, err := c.CreateAccountAndToken(ctx, address, pass)
	if err != nil {
		return nil, nil, "", err
	}
	return acc, tok, pass, nil
}

const lettersDigits = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const strongAlphabet = lettersDigits + "_-!@#$%^&*"

func randString(n int, alphabet string) (string, error) {
	if n <= 0 {
		return "", nil
	}
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		x, err := cryptoRandInt(len(alphabet))
		if err != nil {
			return "", err
		}
		b[i] = alphabet[x]
	}
	return string(b), nil
}

func cryptoRandInt(n int) (int, error) {
	if n <= 0 {
		return 0, errors.New("invalid bound")
	}
	var b [8]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			return 0, err
		}
		v := uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
			uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
		if n <= 0 {
			return 0, errors.New("invalid bound")
		}
		if m := (1 << 63) - (1<<63)%uint64(n); v < uint64(m) {
			return int(v % uint64(n)), nil
		}
	}
}

// DeleteAllMessages deletes all messages in the mailbox (sequentially, respecting rate limiter).
func (c *Client) DeleteAllMessages(ctx context.Context) error {
	return c.IterateMessages(ctx, func(m Message) error {
		return c.DeleteMessage(ctx, m.ID)
	})
}

// DownloadAllAttachments downloads all attachments of a message to dir and returns saved file paths.
// Files are written by streaming and won't be fully loaded into memory.
func (c *Client) DownloadAllAttachments(ctx context.Context, messageID string, dir string) ([]string, error) {
	msg, err := c.GetMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var saved []string
	used := map[string]int{}
	for _, att := range msg.Attachments {
		name := att.Filename
		if name == "" {
			name = "attachment"
		}
		base := sanitizeFilename(name)
		fn := base
		if used[base] > 0 {
			fn = fmt.Sprintf("%s_%d", base, used[base])
		}
		used[base]++
		path := filepath.Join(dir, fn)
		f, err := os.Create(path)
		if err != nil {
			return saved, err
		}
		if err := c.DownloadByURL(ctx, att.DownloadURL, f); err != nil {
			f.Close()
			return saved, err
		}
		f.Close()
		saved = append(saved, path)
	}
	return saved, nil
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	return s
}
