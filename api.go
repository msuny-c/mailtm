package mailtm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) absURL(p string) (string, error) {
	// Resolve relative or absolute paths while preserving query strings.
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p, nil
	}
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	rel, err := url.Parse(p)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(rel).String(), nil
}

func (c *Client) doJSON(ctx context.Context, method, p string, body any, out any) error {
	u, err := c.absURL(p)
	if err != nil {
		return err
	}
	var rc io.ReadCloser
	var ct string
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rc = io.NopCloser(strings.NewReader(string(b)))
		ct = "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rc)
	if err != nil {
		return err
	}
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseHTTPError(resp)
	}
	defer resp.Body.Close()
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(out)
}

// ListDomains retrieves a paginated list of available domains.
// Pass page=0 to get the first page.
func (c *Client) ListDomains(ctx context.Context, page int) (Collection[Domain], error) {
	var coll Collection[Domain]
	p := "/domains"
	if page > 0 {
		p = fmt.Sprintf("%s?page=%d", p, page)
	}
	err := c.doJSON(ctx, http.MethodGet, p, nil, &coll)
	return coll, err
}

// GetDomain retrieves details for a specific domain by its ID.
func (c *Client) GetDomain(ctx context.Context, id string) (*Domain, error) {
	var d Domain
	err := c.doJSON(ctx, http.MethodGet, "/domains/"+url.PathEscape(id), nil, &d)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// CreateAccount creates a new account with the specified email address and password.
func (c *Client) CreateAccount(ctx context.Context, address, password string) (*Account, error) {
	payload := map[string]string{"address": address, "password": password}
	var acc Account
	err := c.doJSON(ctx, http.MethodPost, "/accounts", payload, &acc)
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

// Token exchanges user credentials for a JWT authentication token.
func (c *Client) Token(ctx context.Context, address, password string) (*Token, error) {
	payload := map[string]string{"address": address, "password": password}
	var tok Token
	err := c.doJSON(ctx, http.MethodPost, "/token", payload, &tok)
	if err != nil {
		return nil, err
	}
	return &tok, nil
}

// GetAccount retrieves account details by account ID.
func (c *Client) GetAccount(ctx context.Context, id string) (*Account, error) {
	var acc Account
	err := c.doJSON(ctx, http.MethodGet, "/accounts/"+url.PathEscape(id), nil, &acc)
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

// DeleteAccount permanently deletes an account by its ID.
func (c *Client) DeleteAccount(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/accounts/"+url.PathEscape(id), nil, nil)
}

// Me retrieves the current authenticated user's account information.
func (c *Client) Me(ctx context.Context) (*Account, error) {
	var acc Account
	err := c.doJSON(ctx, http.MethodGet, "/me", nil, &acc)
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

// ListMessages retrieves a paginated list of messages in the mailbox.
// Pass page=0 to get the first page.
func (c *Client) ListMessages(ctx context.Context, page int) (Collection[Message], error) {
	var coll Collection[Message]
	p := "/messages"
	if page > 0 {
		p = fmt.Sprintf("%s?page=%d", p, page)
	}
	err := c.doJSON(ctx, http.MethodGet, p, nil, &coll)
	return coll, err
}

// GetMessage retrieves the full details of a specific message by its ID.
func (c *Client) GetMessage(ctx context.Context, id string) (*Message, error) {
	var m Message
	err := c.doJSON(ctx, http.MethodGet, "/messages/"+url.PathEscape(id), nil, &m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DeleteMessage permanently deletes a message by its ID.
func (c *Client) DeleteMessage(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/messages/"+url.PathEscape(id), nil, nil)
}

// MarkRead marks a message as read by its ID.
func (c *Client) MarkRead(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodPatch, "/messages/"+url.PathEscape(id), nil, nil)
}

// GetSource retrieves the raw email source by source ID.
func (c *Client) GetSource(ctx context.Context, sourceID string) (*Source, error) {
	var s Source
	err := c.doJSON(ctx, http.MethodGet, "/sources/"+url.PathEscape(sourceID), nil, &s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// DownloadByURL downloads content from a URL and writes it to the provided writer.
// The URL can be relative (resolved against the base URL) or absolute.
func (c *Client) DownloadByURL(ctx context.Context, rawURL string, w io.Writer) error {
	u, err := c.absURL(rawURL)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseHTTPError(resp)
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	return err
}

// IterateMessages iterates through all messages in the mailbox, calling the provided function for each message.
// If the function returns an error, iteration stops and the error is returned.
func (c *Client) IterateMessages(ctx context.Context, fn func(Message) error) error {
	u := "/messages?page=1"
	for {
		var coll Collection[Message]
		if err := c.doJSON(ctx, http.MethodGet, u, nil, &coll); err != nil {
			return err
		}
		for _, m := range coll.Member {
			if err := fn(m); err != nil {
				return err
			}
		}
		if coll.View == nil || coll.View.Next == "" {
			return nil
		}
		u = coll.View.Next
	}
}
