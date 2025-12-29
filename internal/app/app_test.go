package app

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/mitsuhiko/gh-issue-sync/internal/config"
	"github.com/mitsuhiko/gh-issue-sync/internal/ghcli"
	"github.com/mitsuhiko/gh-issue-sync/internal/issue"
	"github.com/mitsuhiko/gh-issue-sync/internal/paths"
)

func TestApplyMapping(t *testing.T) {
	parent := issue.IssueRef("T1")
	item := issue.Issue{
		Number: issue.IssueNumber("T2"),
		Title:  "Test",
		Body:   "Refs #T1 and #T10\n",
		Parent: &parent,
		BlockedBy: []issue.IssueRef{
			"T1",
			"99",
		},
	}
	mapping := map[string]string{"T1": "123"}
	changed := applyMapping(&item, mapping)
	if !changed {
		t.Fatalf("expected mapping to report change")
	}
	if item.Body != "Refs #123 and #T10\n" {
		t.Fatalf("unexpected body: %q", item.Body)
	}
	if item.Parent == nil || item.Parent.String() != "123" {
		t.Fatalf("unexpected parent: %v", item.Parent)
	}
	if got := item.BlockedBy[0].String(); got != "123" {
		t.Fatalf("unexpected blocked_by mapping: %s", got)
	}
}

func TestApplyMappingHexIDs(t *testing.T) {
	// Test with hex-style local IDs (e.g., T1a2b3c4d)
	parent := issue.IssueRef("Tabc12345")
	item := issue.Issue{
		Number: issue.IssueNumber("T99887766"),
		Title:  "Depends on #Tabc12345",
		Body:   "See #Tabc12345 for details. Also #Tdeadbeef is related.\n",
		Parent: &parent,
		BlockedBy: []issue.IssueRef{
			"Tabc12345",
			"Tdeadbeef",
		},
	}
	mapping := map[string]string{
		"Tabc12345": "100",
		"Tdeadbeef": "200",
	}
	changed := applyMapping(&item, mapping)
	if !changed {
		t.Fatalf("expected mapping to report change")
	}
	if item.Title != "Depends on #100" {
		t.Fatalf("unexpected title: %q", item.Title)
	}
	if item.Body != "See #100 for details. Also #200 is related.\n" {
		t.Fatalf("unexpected body: %q", item.Body)
	}
	if item.Parent == nil || item.Parent.String() != "100" {
		t.Fatalf("unexpected parent: %v", item.Parent)
	}
	if got := item.BlockedBy[0].String(); got != "100" {
		t.Fatalf("unexpected blocked_by[0] mapping: %s", got)
	}
	if got := item.BlockedBy[1].String(); got != "200" {
		t.Fatalf("unexpected blocked_by[1] mapping: %s", got)
	}
}

func TestApplyMappingNoChange(t *testing.T) {
	item := issue.Issue{
		Number: issue.IssueNumber("T1"),
		Title:  "No references here",
		Body:   "Just plain text\n",
	}
	mapping := map[string]string{"Tabc12345": "100"}
	changed := applyMapping(&item, mapping)
	if changed {
		t.Fatalf("expected no change")
	}
}

func TestNewIssueFromEditor(t *testing.T) {
	root := t.TempDir()
	p := paths.New(root)
	if err := p.EnsureLayout(); err != nil {
		t.Fatalf("layout: %v", err)
	}
	if err := config.Save(p.ConfigPath, config.Default("owner", "repo")); err != nil {
		t.Fatalf("config: %v", err)
	}

	var capturedNumber issue.IssueNumber
	previousInteractive := runInteractiveCommand
	runInteractiveCommand = func(ctx context.Context, command string, args ...string) error {
		if len(args) == 0 {
			t.Fatalf("expected editor path")
		}
		// Read the temp file to get the generated issue number
		tempIssue, err := issue.ParseFile(args[len(args)-1])
		if err != nil {
			t.Fatalf("parse temp issue: %v", err)
		}
		capturedNumber = tempIssue.Number
		payload, err := issue.Render(issue.Issue{
			Number: capturedNumber,
			Title:  "Edited Title",
			State:  "open",
			Body:   "Hello\n",
		})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if err := os.WriteFile(args[len(args)-1], []byte(payload), 0o644); err != nil {
			t.Fatalf("write editor payload: %v", err)
		}
		return nil
	}
	t.Cleanup(func() { runInteractiveCommand = previousInteractive })
	t.Setenv("EDITOR", "true")

	application := New(root, ghcli.ExecRunner{}, io.Discard, io.Discard)
	if err := application.NewIssue(context.Background(), "", NewOptions{Edit: true}); err != nil {
		t.Fatalf("new issue: %v", err)
	}

	// Find the created issue file (number is random)
	entries, err := os.ReadDir(p.OpenDir)
	if err != nil {
		t.Fatalf("read open dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 issue file, got %d", len(entries))
	}
	parsed, err := issue.ParseFile(p.OpenDir + "/" + entries[0].Name())
	if err != nil {
		t.Fatalf("parse issue: %v", err)
	}
	if parsed.Title != "Edited Title" {
		t.Fatalf("unexpected title: %q", parsed.Title)
	}
	if !parsed.Number.IsLocal() {
		t.Fatalf("expected local issue number, got %q", parsed.Number)
	}
}
