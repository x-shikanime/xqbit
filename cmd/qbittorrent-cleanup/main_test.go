package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTorrents_withFilter(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter") != "errored" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode([]Torrent{
			{Hash: "aaa", State: "missingFiles", Name: "Torrent A"},
			{Hash: "bbb", State: "error", Name: "Torrent B"},
		})
	})
	defer ts.Close()

	client, _ := NewClient(ts.URL)
	torrents, err := client.GetTorrents(context.Background(), "errored")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(torrents) != 2 {
		t.Fatalf("expected 2 torrents, got %d", len(torrents))
	}
}

func TestGetErroredTorrents(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter") != "errored" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode([]Torrent{
			{Hash: "aaa", State: "missingFiles", Name: "Torrent A"},
		})
	})
	defer ts.Close()

	client, _ := NewClient(ts.URL)
	torrents, err := client.GetErroredTorrents(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(torrents) != 1 {
		t.Fatalf("expected 1 torrent, got %d", len(torrents))
	}
	if torrents[0].State != "missingFiles" {
		t.Fatalf("expected state missingFiles, got %s", torrents[0].State)
	}
}

func TestGetMissingFilesTorrents(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Called without filter — fetches all torrents
		if r.URL.Query().Get("filter") != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode([]Torrent{
			{Hash: "aaa", State: "missingFiles", Name: "Torrent A"},
			{Hash: "bbb", State: "downloading", Name: "Torrent B"},
			{Hash: "ccc", State: "missingFiles", Name: "Torrent C"},
			{Hash: "ddd", State: "error", Name: "Torrent D"},
		})
	})
	defer ts.Close()

	client, _ := NewClient(ts.URL)
	torrents, err := client.GetMissingFilesTorrents(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(torrents) != 2 {
		t.Fatalf("expected 2 missingFiles torrents, got %d", len(torrents))
	}
	if torrents[0].Name != "Torrent A" || torrents[1].Name != "Torrent C" {
		t.Fatal("unexpected torrent order or names")
	}
}

func TestGetMissingFilesTorrents_none(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Torrent{
			{Hash: "bbb", State: "downloading", Name: "Torrent B"},
			{Hash: "ddd", State: "error", Name: "Torrent D"},
		})
	})
	defer ts.Close()

	client, _ := NewClient(ts.URL)
	torrents, err := client.GetMissingFilesTorrents(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(torrents) != 0 {
		t.Fatalf("expected 0 torrents, got %d", len(torrents))
	}
}

func TestGetTorrents_emptyFilter(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// No filter param — fetches all
		if r.URL.Query().Get("filter") != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode([]Torrent{
			{Hash: "aaa", State: "downloading", Name: "All"},
		})
	})
	defer ts.Close()

	client, _ := NewClient(ts.URL)
	torrents, err := client.GetTorrents(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(torrents) != 1 {
		t.Fatalf("expected 1 torrent, got %d", len(torrents))
	}
}

// newTestServer starts a test HTTP server that verifies login then delegates
// to the given handler for subsequent requests.
func newTestServer(t *testing.T, infoHandler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, "Ok.")
		case "/api/v2/torrents/info":
			infoHandler(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
}
