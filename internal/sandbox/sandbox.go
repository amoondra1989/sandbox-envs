package sandbox

import (
	"context"
	"errors"

	"github.com/amoondra1989/sandbox-envs/pkg/sandboxclient"
)

// ErrNotFound means the sandbox ID is unknown to this process.
var ErrNotFound = errors.New("sandbox not found")

// Sandbox is the domain API backed by Podman (or mocks in tests).
type Sandbox interface {
	Create(ctx context.Context, opts sandboxclient.CreateOptions) (string, error)
	Destroy(ctx context.Context, sandboxID string) error
	Exec(ctx context.Context, sandboxID string, command string, opts sandboxclient.ExecOptions) (*sandboxclient.ExecResult, error)
	ReadFile(ctx context.Context, sandboxID string, path string) ([]byte, error)
	WriteFile(ctx context.Context, sandboxID string, path string, content []byte) error
	HealthCheck(ctx context.Context, sandboxID string) error
}
