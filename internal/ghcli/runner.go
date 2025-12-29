package ghcli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
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
		return stdout.String(), fmt.Errorf("%s %v failed: %w: %s", name, args, err, stderr.String())
	}
	return stdout.String(), nil
}
