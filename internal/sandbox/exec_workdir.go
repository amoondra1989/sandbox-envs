package sandbox

import (
	"errors"
	"fmt"
	"strings"

	"github.com/amoondra1989/sandbox-envs/pkg/sandboxclient"
)

// ErrInvalidExecWorkingDir indicates a bad optional working directory (syntax or not a directory inside the container).
var ErrInvalidExecWorkingDir = errors.New("invalid exec working directory")

// EffectiveWorkingDir returns opts.Cwd if non-empty after trim, else opts.WorkingDir (legacy). Both may be empty.
func EffectiveWorkingDir(opts sandboxclient.ExecOptions) string {
	if w := strings.TrimSpace(opts.Cwd); w != "" {
		return w
	}
	return strings.TrimSpace(opts.WorkingDir)
}

// ValidateExecWorkingDirSyntax checks non-empty dir is safe to use as a container path (absolute, no "..").
// Empty dir is valid (means omit workdir — caller does not apply CWD constraint).
func ValidateExecWorkingDirSyntax(dir string) error {
	if dir == "" {
		return nil
	}
	if err := validateSandboxPath(dir); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidExecWorkingDir, err)
	}
	return nil
}
