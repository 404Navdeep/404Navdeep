package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchLatestPushEventSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Fatalf("expected Authorization header to be set, got %q", got)
		}
		if got := r.URL.Path; got != "/users/404Navdeep/events/public" {
			t.Fatalf("unexpected request path: %s", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"type":"PushEvent","repo":{"name":"404Navdeep/404Navdeep"},"payload":{"head":"abcdef1234567890"}}]`))
	}))
	defer server.Close()

	lastRepo, lastCommit, err := fetchLatestPushEvent(server.Client(), server.URL, "404Navdeep", "test-token")
	if err != nil {
		t.Fatalf("fetchLatestPushEvent returned unexpected error: %v", err)
	}
	if lastRepo != "404Navdeep/404Navdeep" {
		t.Fatalf("unexpected repo: %s", lastRepo)
	}
	if !strings.Contains(lastCommit, "abcdef1") {
		t.Fatalf("unexpected commit markdown: %s", lastCommit)
	}
}

func TestFetchLatestPushEventStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer server.Close()

	_, _, err := fetchLatestPushEvent(server.Client(), server.URL, "404Navdeep", "")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected status code in error, got: %v", err)
	}
}
