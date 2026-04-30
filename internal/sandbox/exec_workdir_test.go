package sandbox

import (
	"errors"
	"testing"

	"github.com/amoondra1989/sandbox-envs/pkg/sandboxclient"
)

func TestEffectiveWorkingDir(t *testing.T) {
	t.Parallel()
	if got := EffectiveWorkingDir(sandboxclient.ExecOptions{}); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := EffectiveWorkingDir(sandboxclient.ExecOptions{Cwd: "/a", WorkingDir: "/b"}); got != "/a" {
		t.Fatalf("want /a got %q", got)
	}
	if got := EffectiveWorkingDir(sandboxclient.ExecOptions{WorkingDir: "/b"}); got != "/b" {
		t.Fatalf("want /b got %q", got)
	}
	if got := EffectiveWorkingDir(sandboxclient.ExecOptions{Cwd: "  ", WorkingDir: "/z"}); got != "/z" {
		t.Fatalf("want /z got %q", got)
	}
}

func TestValidateExecWorkingDirSyntax(t *testing.T) {
	t.Parallel()
	if err := ValidateExecWorkingDirSyntax(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateExecWorkingDirSyntax("/workspace"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateExecWorkingDirSyntax("rel"); err == nil || !errors.Is(err, ErrInvalidExecWorkingDir) {
		t.Fatalf("got %v", err)
	}
	if err := ValidateExecWorkingDirSyntax("/x/../y"); err == nil {
		t.Fatal("expected err")
	}
}
