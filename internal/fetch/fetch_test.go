package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func init() { backoff = func(int) time.Duration { return time.Millisecond } }

// A server error is the case this package exists for: fresh release assets
// answer 502, 503 and 504 while they propagate, which is exactly when a release
// dispatch arrives.
func TestARetryableStatusIsRetried(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			http.Error(w, "not yet", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	resp, err := Get(context.Background(), server.Client(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if calls != 3 {
		t.Errorf("made %d requests, want 3", calls)
	}
}

// A 4xx never gets better by asking again, and retrying only delays the message
// that says which provider is wrong.
func TestAClientErrorIsNotRetried(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.NotFound(w, r)
	}))
	defer server.Close()

	resp, err := Get(context.Background(), server.Client(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status %s, want 404", resp.Status)
	}
	if calls != 1 {
		t.Errorf("made %d requests, want 1", calls)
	}
}

func TestGivingUpSaysWhat(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer server.Close()

	if _, err := Get(context.Background(), server.Client(), server.URL, nil); err == nil {
		t.Fatal("a server that is always down was reported as working")
	}
	if calls != Attempts {
		t.Errorf("made %d requests, want %d", calls, Attempts)
	}
}

func TestHeadersReachEveryAttempt(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		if len(seen) < 2 {
			http.Error(w, "not yet", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	resp, err := Get(context.Background(), server.Client(), server.URL, http.Header{"Authorization": {"Bearer t"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	for i, got := range seen {
		if got != "Bearer t" {
			t.Errorf("attempt %d sent %q", i+1, got)
		}
	}
}
