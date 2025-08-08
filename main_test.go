package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchEvents_OK(t *testing.T) {
	// Fake GitHub endpoint
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		if !strings.HasPrefix(r.URL.Path, "/users/torvalds/events") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		// Return a minimal, valid events array
		resp := []map[string]any{
			{
				"type":       "PushEvent",
				"created_at": time.Now().UTC().Format(time.RFC3339),
				"repo":       map[string]any{"name": "alice/repo"},
				"payload":    map[string]any{"size": 2},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// Override the global eventsURL
	restore := eventsURL
	eventsURL = srv.URL + "/users/%s/events"
	defer func() { eventsURL = restore }()

	evs, err := fetchEvents("torvalds")
	if err != nil {
		t.Fatalf("fetchEvents error: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	if gotUA == "" {
		t.Fatal("expected User-Agent header to be set")
	}
	line, ok := formatEvent(evs[0])
	if !ok || !strings.Contains(line, "Pushed 2 commit(s) to alice/repo") {
		t.Fatalf("unexpected formatted line: %q ok=%v", line, ok)
	}
}

func TestFetchEvents_UserNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	restore := eventsURL
	eventsURL = srv.URL + "/users/%s/events"
	defer func() { eventsURL = restore }()

	_, err := fetchEvents("nope")
	if err == nil || !strings.Contains(err.Error(), "user not found") {
		t.Fatalf("expected user not found error, got %v", err)
	}
}

func TestFetchEvents_RateLimited(t *testing.T) {
	reset := time.Now().Add(5 * time.Minute).Unix()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", reset))
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()
	restore := eventsURL
	eventsURL = srv.URL + "/users/%s/events"
	defer func() { eventsURL = restore }()

	_, err := fetchEvents("someone")
	if err == nil || !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Fatalf("expected rate limit error, got %v", err)
	}
}

func TestFetchEvents_GenericAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()
	restore := eventsURL
	eventsURL = srv.URL + "/users/%s/events"
	defer func() { eventsURL = restore }()

	_, err := fetchEvents("anyone")
	if err == nil || !strings.Contains(err.Error(), "github api error") {
		t.Fatalf("expected generic api error, got %v", err)
	}
}

func TestTitleCase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"opened", "Opened"},
		{"CLOSED", "Closed"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := titleCase(tc.in); got != tc.want {
			t.Fatalf("titleCase(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseUnix(t *testing.T) {
	ts, err := parseUnix("1710000000") // known epoch
	if err != nil {
		t.Fatalf("parseUnix error: %v", err)
	}
	if ts.IsZero() {
		t.Fatal("parseUnix returned zero time")
	}
	// sanity: must be close to 2024-03-ish (don’t assert exact timezone)
	if ts.Year() < 2023 || ts.Year() > time.Now().Year()+1 {
		t.Fatalf("unexpected year from parseUnix: %v", ts)
	}
}

func mustRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestFormatEvent_Push(t *testing.T) {
	ev := Event{
		Type: "PushEvent",
		Repo: struct {
			Name string `json:"name"`
		}{Name: "alice/repo"},
		Payload: mustRaw(struct {
			Size int `json:"size"`
		}{Size: 3}),
	}
	got, ok := formatEvent(ev)
	if !ok {
		t.Fatal("formatEvent returned ok=false for PushEvent")
	}
	want := "Pushed 3 commit(s) to alice/repo"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatEvent_Issues(t *testing.T) {
	ev := Event{
		Type: "IssuesEvent",
		Repo: struct {
			Name string `json:"name"`
		}{Name: "alice/repo"},
		Payload: mustRaw(struct {
			Action string `json:"action"`
			Issue  struct {
				Number int    `json:"number"`
				Title  string `json:"title"`
			} `json:"issue"`
		}{
			Action: "opened",
			Issue: struct {
				Number int    `json:"number"`
				Title  string `json:"title"`
			}{Number: 42, Title: "Bug"},
		}),
	}
	got, ok := formatEvent(ev)
	if !ok {
		t.Fatal("formatEvent returned ok=false for IssuesEvent")
	}
	want := `Opened an issue #42 “Bug” in alice/repo`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatEvent_PR(t *testing.T) {
	ev := Event{
		Type: "PullRequestEvent",
		Repo: struct {
			Name string `json:"name"`
		}{Name: "alice/repo"},
		Payload: mustRaw(struct {
			Action      string `json:"action"`
			PullRequest struct {
				Number int    `json:"number"`
				Title  string `json:"title"`
			} `json:"pull_request"`
		}{
			Action: "closed",
			PullRequest: struct {
				Number int    `json:"number"`
				Title  string `json:"title"`
			}{Number: 7, Title: "Feature"},
		}),
	}
	got, ok := formatEvent(ev)
	if !ok {
		t.Fatal("formatEvent returned ok=false for PullRequestEvent")
	}
	want := `Closed a pull request #7 “Feature” in alice/repo`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatEvent_WatchStarted(t *testing.T) {
	ev := Event{
		Type: "WatchEvent",
		Repo: struct {
			Name string `json:"name"`
		}{Name: "alice/repo"},
		Payload: mustRaw(struct {
			Action string `json:"action"`
		}{Action: "started"}),
	}
	got, ok := formatEvent(ev)
	if !ok {
		t.Fatal("formatEvent returned ok=false for WatchEvent started")
	}
	want := "Starred alice/repo"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatEvent_Fork(t *testing.T) {
	ev := Event{
		Type: "ForkEvent",
		Repo: struct {
			Name string `json:"name"`
		}{Name: "alice/repo"},
		Payload: mustRaw(struct {
			Forkee struct {
				FullName string `json:"full_name"`
			} `json:"forkee"`
		}{
			Forkee: struct {
				FullName string `json:"full_name"`
			}{FullName: "bob/repo-fork"},
		}),
	}
	got, ok := formatEvent(ev)
	if !ok {
		t.Fatal("formatEvent returned ok=false for ForkEvent")
	}
	want := "Forked alice/repo → bob/repo-fork"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatEvent_GenericTypes(t *testing.T) {
	tests := []struct {
		typ  string
		want string
	}{
		{"CreateEvent", "Created something in alice/repo"},
		{"DeleteEvent", "Deleted something in alice/repo"},
		{"ReleaseEvent", "Published or edited a release in alice/repo"},
		{"PullRequestReviewCommentEvent", "Commented on a PR review in alice/repo"},
		{"IssueCommentEvent", "Commented on an issue in alice/repo"},
	}
	for _, tc := range tests {
		ev := Event{
			Type: tc.typ,
			Repo: struct {
				Name string `json:"name"`
			}{Name: "alice/repo"},
		}
		got, ok := formatEvent(ev)
		if !ok {
			t.Fatalf("formatEvent returned ok=false for %s", tc.typ)
		}
		if got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.typ, got, tc.want)
		}
	}
}

func TestFormatEvent_UnknownType(t *testing.T) {
	ev := Event{
		Type: "UnknownEvent",
		Repo: struct {
			Name string `json:"name"`
		}{Name: "alice/repo"},
	}
	if got, ok := formatEvent(ev); ok || got != "" {
		t.Fatalf("expected skip for unknown type, got ok=%v line=%q", ok, got)
	}
}
