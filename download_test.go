package mailtm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloadByURL(t *testing.T) {
	const payload = "HELLO-EML"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, payload)
	}))
	defer s.Close()

	c, _ := New(WithBaseURL(s.URL))
	var b strings.Builder
	if err := c.DownloadByURL(context.Background(), s.URL+"/eml/1", &b); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if b.String() != payload {
		t.Fatalf("bad payload: %q", b.String())
	}
}
