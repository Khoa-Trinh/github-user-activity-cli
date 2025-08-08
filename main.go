package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var eventsURL = "https://api.github.com/users/%s/events"

const userAgent = "github-activity-cli/1.0"

type Event struct {
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Repo      struct {
		Name string `json:"name"`
	} `json:"repo"`
	// payload is dynamic per event type; we only decode fields we need
	Payload json.RawMessage `json:"payload"`
}

type PushPayload struct {
	Size int `json:"size"`
}

type IssuesPayload struct {
	Action string `json:"action"`
	Issue  struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	} `json:"issue"`
}

type PRPayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	} `json:"pull_request"`
}

type WatchPayload struct {
	Action string `json:"action"`
}

type ForkPayload struct {
	Forkee struct {
		FullName string `json:"full_name"`
	} `json:"forkee"`
}

func main() {
	eventType := flag.String("type", "", "Filter by event type (e.g., PushEvent, IssuesEvent). Leave blank for all.")
	limit := flag.Int("n", 30, "Max number of events to show (1-100).")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] <github-username>\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Options:")
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), `
Examples:
  github-activity torvalds
  github-activity --type=PushEvent --n=10 kamranahmedse`)
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	username := flag.Arg(0)
	if *limit < 1 {
		*limit = 1
	}
	if *limit > 100 {
		*limit = 100
	}

	events, err := fetchEvents(username)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	if len(events) == 0 {
		fmt.Println("No recent public activity.")
		return
	}

	count := 0
	for _, ev := range events {
		if *eventType != "" && ev.Type != *eventType {
			continue
		}
		line, ok := formatEvent(ev)
		if !ok {
			continue // skip unknown/boring events
		}
		fmt.Println("- " + line)
		count++
		if count >= *limit {
			break
		}
	}

	if count == 0 {
		if *eventType != "" {
			fmt.Printf("No events of type %q found.\n", *eventType)
		} else {
			fmt.Println("No printable events found.")
		}
	}
}

func fetchEvents(username string) ([]Event, error) {
	url := fmt.Sprintf(eventsURL, username)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	// If you have a token, uncomment to raise your rate limit:
	// req.Header.Set("Authorization", "Bearer "+os.Getenv("GITHUB_TOKEN"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("user not found")
	}
	if resp.StatusCode == http.StatusForbidden {
		// likely rate limited
		if rl := resp.Header.Get("X-RateLimit-Remaining"); rl == "0" {
			reset := resp.Header.Get("X-RateLimit-Reset")
			msg := "rate limit exceeded; set GITHUB_TOKEN to increase limits"
			if reset != "" {
				if ts, _ := parseUnix(reset); !ts.IsZero() {
					msg += fmt.Sprintf(" (resets at %s)", ts.Local().Format(time.RFC1123))
				}
			}
			return nil, errors.New(msg)
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github api error: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var events []Event
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&events); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}
	return events, nil
}

func parseUnix(s string) (time.Time, error) {
	// GitHub gives unix seconds
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, 0), nil
}

func formatEvent(ev Event) (string, bool) {
	repo := ev.Repo.Name
	switch ev.Type {
	case "PushEvent":
		var p PushPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return "", false
		}
		return fmt.Sprintf("Pushed %d commit(s) to %s", p.Size, repo), true

	case "IssuesEvent":
		var p IssuesPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return "", false
		}
		action := strings.ToLower(p.Action)
		return fmt.Sprintf("%s an issue #%d “%s” in %s", titleCase(action), p.Issue.Number, p.Issue.Title, repo), true

	case "PullRequestEvent":
		var p PRPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return "", false
		}
		action := strings.ToLower(p.Action)
		return fmt.Sprintf("%s a pull request #%d “%s” in %s", titleCase(action), p.PullRequest.Number, p.PullRequest.Title, repo), true

	case "WatchEvent":
		var p WatchPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return "", false
		}
		if strings.ToLower(p.Action) == "started" {
			return fmt.Sprintf("Starred %s", repo), true
		}
		return fmt.Sprintf("Watch event on %s", repo), true

	case "ForkEvent":
		var p ForkPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return "", false
		}
		target := repo
		if p.Forkee.FullName != "" {
			target = p.Forkee.FullName
		}
		return fmt.Sprintf("Forked %s → %s", ev.Repo.Name, target), true

	case "CreateEvent":
		// repo/branch/tag created; keep it simple
		return fmt.Sprintf("Created something in %s", repo), true
	case "DeleteEvent":
		return fmt.Sprintf("Deleted something in %s", repo), true
	case "ReleaseEvent":
		return fmt.Sprintf("Published or edited a release in %s", repo), true
	case "PullRequestReviewCommentEvent":
		return fmt.Sprintf("Commented on a PR review in %s", repo), true
	case "IssueCommentEvent":
		return fmt.Sprintf("Commented on an issue in %s", repo), true
	default:
		// Too many types; skip the obscure ones for brevity
		return "", false
	}
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}
