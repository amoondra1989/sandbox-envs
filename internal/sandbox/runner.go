package sandbox

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

// Runner runs external commands (Podman CLI). Implemented by ExecRunner in production.
type Runner interface {
	Run(ctx context.Context, bin string, args ...string) (stdout []byte, stderr []byte, exitCode int, err error)
}

// ExecRunner implements Runner using os/exec.
type ExecRunner struct{}

// Run executes bin with args. On success exitCode is 0; on *exec.ExitError, exitCode is the process exit code and err is nil.
// If the command fails to start, exitCode is -1 and err is non-nil.
func (ExecRunner) Run(ctx context.Context, bin string, args ...string) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.Bytes()
	errOut := stderr.Bytes()
	if err == nil {
		return out, errOut, 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out, errOut, ee.ExitCode(), nil
	}
	return out, errOut, -1, err
}
