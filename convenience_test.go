package mailtm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestCreateRandomAccount_Flow(t *testing.T) {
	// Mock API with 2 pages of domains, then accounts/token
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/domains":
			page := r.URL.Query().Get("page")
			if page == "" || page == "1" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"hydra:member": []map[string]any{
						{"id":"d1","domain":"example1.tm","isActive":true,"isPrivate":false},
					},
					"hydra:totalItems": 2,
					"hydra:view": map[string]any{"hydra:next": "/domains?page=2"},
				})
				return
			}
			if page == "2" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"hydra:member": []map[string]any{
						{"id":"d2","domain":"example2.tm","isActive":true,"isPrivate":false},
					},
					"hydra:totalItems": 2,
				})
				return
			}
		case r.URL.Path == "/accounts" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":"a1","address":"placeholder@example1.tm",
			})
			return
		case r.URL.Path == "/token" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":"a1","token":"tkn",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer s.Close()

	c, _ := New(WithBaseURL(s.URL))

	acc, tok, pass, err := c.CreateRandomAccount(context.Background(), RandomAccountOptions{
		UsernamePrefix: "u",
		UsernameLen:    6,
		PasswordLen:    10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok == nil || tok.Token != "tkn" || acc == nil {
		t.Fatalf("bad token/account: acc=%v tok=%v", acc, tok)
	}
	if len(pass) != 10 {
		t.Fatalf("password length mismatch: %d", len(pass))
	}
}

func TestDownloadAllAttachments(t *testing.T) {
	// message with two attachments (same filename triggers suffixing)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/messages/m1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "m1",
				"from": map[string]any{"name":"", "address":""},
				"attachments": []map[string]any{
					{"id":"a1","filename":"file.txt","downloadUrl":"/dl/1"},
					{"id":"a2","filename":"file.txt","downloadUrl":"/dl/2"},
				},
			})
			return
		case r.URL.Path == "/dl/1":
			w.Write([]byte("A"))
			return
		case r.URL.Path == "/dl/2":
			w.Write([]byte("B"))
			return
		}
		http.NotFound(w, r)
	}))
	defer s.Close()

	c, _ := New(WithBaseURL(s.URL))
	tmp := t.TempDir()
	paths, err := c.DownloadAllAttachments(context.Background(), "m1", tmp)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 files, got %d", len(paths))
	}
	if filepath.Base(paths[0]) == filepath.Base(paths[1]) {
		t.Fatalf("filenames must be unique: %v", paths)
	}
}
