package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"equal to max", "hello", 5, "hello"},
		{"longer than max", "hello world", 5, "hell…"},
		{"multibyte safe", "あいうえおかき", 4, "あいう…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.in, tt.max); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
			if n := len([]rune(truncate(tt.in, tt.max))); n > tt.max {
				t.Errorf("truncate(%q, %d) length = %d runes, want <= %d", tt.in, tt.max, n, tt.max)
			}
		})
	}
}

func TestTokenName(t *testing.T) {
	t.Run("uses short repo names joined by comma", func(t *testing.T) {
		got := tokenName("2026-06-26", []string{"owner/repo-a", "owner/repo-b"})
		want := "2026-06-26 repo-a,repo-b"
		if got != want {
			t.Errorf("tokenName = %q, want %q", got, want)
		}
	})

	t.Run("truncated to 40 chars", func(t *testing.T) {
		repos := []string{"owner/aaaaaaaaaa", "owner/bbbbbbbbbb", "owner/cccccccccc"}
		got := tokenName("2026-06-26", repos)
		if n := len([]rune(got)); n > maxTokenNameLen {
			t.Errorf("tokenName length = %d, want <= %d (%q)", n, maxTokenNameLen, got)
		}
		if !strings.HasSuffix(got, "…") {
			t.Errorf("expected ellipsis suffix on truncated name, got %q", got)
		}
	})
}

func TestTokenDescription(t *testing.T) {
	got := tokenDescription("2026-06-26", []string{"owner/repo-a", "owner/repo-b"})
	if !strings.Contains(got, "gh CLI") {
		t.Errorf("description should mention the gh CLI, got %q", got)
	}
	for _, r := range []string{"owner/repo-a", "owner/repo-b", "2026-06-26"} {
		if !strings.Contains(got, r) {
			t.Errorf("description %q missing %q", got, r)
		}
	}
}

func TestBuildURL(t *testing.T) {
	repos := []string{"acme/repo-a", "acme/repo-b"}
	ids := []string{"111", "222"}
	link := buildURL("acme", repos, ids, "30", "2026-06-26")

	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("buildURL produced an unparseable URL: %v", err)
	}
	if got := u.Scheme + "://" + u.Host + u.Path; got != baseURL {
		t.Errorf("base URL = %q, want %q", got, baseURL)
	}

	q := u.Query()
	checks := map[string]string{
		"target_name":    "acme",
		"expires_in":     "30",
		"repository_ids": "111,222",
		"contents":       "write",
		"metadata":       "read",
		"issues":         "write",
		"pull_requests":  "write",
		"actions":        "read",
	}
	for k, want := range checks {
		if got := q.Get(k); got != want {
			t.Errorf("query %q = %q, want %q", k, got, want)
		}
	}
	if q.Get("name") == "" {
		t.Error("query name is empty")
	}
	if !strings.Contains(q.Get("description"), "gh CLI") {
		t.Errorf("query description should mention gh CLI, got %q", q.Get("description"))
	}
}

func TestBuildURLWithoutIDs(t *testing.T) {
	// When gh is unavailable we have no repo IDs; repository_ids must be omitted,
	// but the rest of the URL must still be valid.
	link := buildURL("acme", []string{"acme/repo-a"}, nil, "90", "2026-06-26")

	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("buildURL produced an unparseable URL: %v", err)
	}
	q := u.Query()
	if _, ok := q["repository_ids"]; ok {
		t.Errorf("repository_ids should be absent without IDs, got %q", q.Get("repository_ids"))
	}
	if q.Get("target_name") != "acme" {
		t.Errorf("target_name = %q, want acme", q.Get("target_name"))
	}
	if q.Get("contents") != "write" {
		t.Errorf("contents = %q, want write", q.Get("contents"))
	}
}

// Every permission we prefill must use a valid access level.
func TestPermissionsValid(t *testing.T) {
	valid := map[string]bool{"read": true, "write": true, "admin": true}
	for perm, level := range permissions {
		if !valid[level] {
			t.Errorf("permission %q has invalid level %q", perm, level)
		}
	}
}
