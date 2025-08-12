package mailtm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIterateMessages_Pagination(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/messages":
			page := r.URL.Query().Get("page")
			if page == "" || page == "1" {
				resp := map[string]any{
					"hydra:member": []map[string]any{
						{"id": "m1", "subject": "a", "from": map[string]any{"name": "", "address": ""}},
					},
					"hydra:totalItems": 2,
					"hydra:view": map[string]any{
						"hydra:next": "/messages?page=2",
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			if page == "2" {
				resp := map[string]any{
					"hydra:member": []map[string]any{
						{"id": "m2", "subject": "b", "from": map[string]any{"name": "", "address": ""}},
					},
					"hydra:totalItems": 2,
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()

	c, _ := New(WithBaseURL(s.URL))
	var got []string
	if err := c.IterateMessages(context.Background(), func(m Message) error {
		got = append(got, m.ID)
		return nil
	}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if fmt.Sprint(got) != "[m1 m2]" {
		t.Fatalf("unexpected order: %v", got)
	}
}
