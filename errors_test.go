package mailtm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseHTTPError_Hydra(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/ld+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hydra:title":       "Validation Failed",
			"hydra:description": "Bad domain",
		})
	}))
	defer s.Close()

	c, _ := New(WithBaseURL(s.URL))
	err := c.doJSON(context.Background(), http.MethodGet, "/bad", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("expected *HTTPError, got %T", err)
	}
	if he.StatusCode != 422 || he.Title == "" || he.Detail == "" || len(he.Raw) == 0 {
		t.Fatalf("unexpected http error: %+v", he)
	}
}

func TestParseHTTPError_Text(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "teapot", http.StatusTeapot)
	}))
	defer s.Close()

	c, _ := New(WithBaseURL(s.URL))
	err := c.doJSON(context.Background(), http.MethodGet, "/x", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	he := err.(*HTTPError)
	if he.StatusCode != http.StatusTeapot || he.Detail == "" {
		t.Fatalf("bad http error: %+v", he)
	}
}
