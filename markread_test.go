package mailtm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMarkRead_UsesPATCH_NoBody(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.ContentLength > 0 {
			t.Fatalf("expected no body, got %d", r.ContentLength)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	c, _ := New(WithBaseURL(s.URL))
	if err := c.MarkRead(context.Background(), "id123"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
