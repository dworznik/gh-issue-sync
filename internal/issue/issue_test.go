package issue

import (
	"strings"
	"testing"
	"time"
)

func TestParseRenderRoundTrip(t *testing.T) {
	syncedAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	input := strings.TrimSpace(`---
number: 123
title: "Test issue"
labels:
  - bug
assignees:
  - alice
milestone: "v1"
state: open
state_reason: null
blocked_by:
  - 2
synced_at: 2025-01-02T03:04:05Z
---
Body line
`) + "\n"

	parsed, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if parsed.Number != "123" {
		t.Fatalf("expected number 123, got %s", parsed.Number)
	}
	if parsed.Title != "Test issue" {
		t.Fatalf("expected title, got %q", parsed.Title)
	}
	if parsed.SyncedAt == nil || !parsed.SyncedAt.Equal(syncedAt) {
		t.Fatalf("unexpected synced_at")
	}

	rendered, err := Render(parsed)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	parsedAgain, err := Parse([]byte(rendered))
	if err != nil {
		t.Fatalf("parse rendered failed: %v", err)
	}
	if !EqualIgnoringSyncedAt(parsed, parsedAgain) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Fix login bug":          "fix-login-bug",
		"  Weird---Title  ":      "weird-title",
		"Symbols & stuff!":       "symbols-stuff",
		"":                       "",
		"Multiple     spaces":    "multiple-spaces",
		"Already-slugified-text": "already-slugified-text",
	}
	for input, expected := range cases {
		if got := Slugify(input); got != expected {
			t.Fatalf("slugify %q => %q, want %q", input, got, expected)
		}
	}
}
