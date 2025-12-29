package ghcli

import (
	"context"
	"testing"
)

type recordingRunner struct {
	name string
	args []string
}

func (r *recordingRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	return "[]", nil
}

func TestClientAddsRepoFlag(t *testing.T) {
	runner := &recordingRunner{}
	client := NewClient(runner, "octo/repo")

	if _, err := client.ListIssues(context.Background(), "open", nil); err != nil {
		t.Fatalf("list issues: %v", err)
	}

	if runner.name != "gh" {
		t.Fatalf("expected gh invocation, got %q", runner.name)
	}
	if !hasRepoFlag(runner.args, "octo/repo") {
		t.Fatalf("expected --repo octo/repo, got %v", runner.args)
	}
}

func hasRepoFlag(args []string, repo string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--repo" && args[i+1] == repo {
			return true
		}
	}
	return false
}
