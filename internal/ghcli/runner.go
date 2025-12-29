package ghcli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Build a short command summary (don't include long arguments like --body)
		cmdSummary := formatCommandSummary(name, args)
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return stdout.String(), fmt.Errorf("%s failed: %s", cmdSummary, stderrText)
		}
		return stdout.String(), fmt.Errorf("%s failed: %w", cmdSummary, err)
	}
	return stdout.String(), nil
}

// formatCommandSummary returns a short representation of the command,
// truncating long argument values to avoid huge error messages.
func formatCommandSummary(name string, args []string) string {
	if len(args) == 0 {
		return name
	}

	var parts []string
	parts = append(parts, name)

	skipNext := false
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		// Check if this is a flag that takes a long value
		if strings.HasPrefix(arg, "--") && i+1 < len(args) {
			flagName := arg
			flagValue := args[i+1]

			// Truncate long values (like --body)
			if len(flagValue) > 50 {
				parts = append(parts, flagName, truncateArg(flagValue, 50))
				skipNext = true
				continue
			}
		}

		parts = append(parts, arg)
	}

	return strings.Join(parts, " ")
}

func truncateArg(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
