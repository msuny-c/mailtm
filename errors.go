package mailtm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type HTTPError struct {
	StatusCode int
	Title      string
	Detail     string
	Raw        []byte
}

func (e *HTTPError) Error() string {
	if e.Title != "" || e.Detail != "" {
		return fmt.Sprintf("http %d: %s %s", e.StatusCode, e.Title, e.Detail)
	}
	return fmt.Sprintf("http %d", e.StatusCode)
}

func parseHTTPError(resp *http.Response) error {
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = io.CopyN(&buf, resp.Body, 256*1024)
	var tmp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &tmp); err == nil {
		he := &HTTPError{StatusCode: resp.StatusCode, Raw: buf.Bytes()}
		if v, ok := tmp["hydra:title"].(string); ok {
			he.Title = v
		}
		if v, ok := tmp["hydra:description"].(string); ok {
			he.Detail = v
		}
		if he.Title == "" {
			if v, ok := tmp["title"].(string); ok {
				he.Title = v
			}
		}
		if he.Detail == "" {
			if v, ok := tmp["detail"].(string); ok {
				he.Detail = v
			}
		}
		return he
	}
	return &HTTPError{StatusCode: resp.StatusCode, Raw: buf.Bytes(), Detail: buf.String()}
}
