package sandbox

import (
	"context"
	"strings"
	"testing"

	"github.com/amoondra1989/sandbox-envs/pkg/sandboxclient"
)

type fakeRunner struct {
	t        *testing.T
	commands [][]string
}

func (f *fakeRunner) Run(ctx context.Context, bin string, args ...string) ([]byte, []byte, int, error) {
	cmd := append([]string{bin}, args...)
	f.commands = append(f.commands, cmd)
	if len(args) == 0 {
		return nil, nil, -1, nil
	}
	switch args[0] {
	case "run":
		return []byte("cid\n"), nil, 0, nil
	case "rm":
		return nil, nil, 0, nil
	case "exec":
		return []byte("out"), nil, 0, nil
	case "cp":
		return nil, nil, 0, nil
	case "inspect":
		return []byte("true\n"), nil, 0, nil
	default:
		f.t.Fatalf("unexpected podman args: %v", args)
		return nil, nil, -1, nil
	}
}

func TestPodmanCreateAndDestroy(t *testing.T) {
	fr := &fakeRunner{t: t}
	p := NewPodman("myimage:latest", fr)
	p.PodmanBin = "podman"
	ctx := context.Background()
	id, err := p.Create(ctx, sandboxclient.CreateOptions{})
	if err != nil || id == "" {
		t.Fatalf("create: %v id=%q", err, id)
	}
	if err := p.HealthCheck(ctx, id); err != nil {
		t.Fatal(err)
	}
	data, err := p.ReadFile(ctx, id, "/tmp/x")
	if err != nil || string(data) != "out" {
		t.Fatalf("read: %v data=%q", err, data)
	}
	if err := p.WriteFile(ctx, id, "/tmp/y", []byte("z")); err != nil {
		t.Fatal(err)
	}
	res, err := p.Exec(ctx, id, "true", sandboxclient.ExecOptions{})
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("exec: %v %#v", err, res)
	}
	if err := p.Destroy(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := p.lookup(id); err == nil {
		t.Fatal("expected not found after destroy")
	}
}

func TestValidateSandboxPath(t *testing.T) {
	if err := validateSandboxPath("/ok"); err != nil {
		t.Fatal(err)
	}
	if err := validateSandboxPath("rel"); err == nil {
		t.Fatal("expected err")
	}
	if err := validateSandboxPath("/bad/../x"); err == nil {
		t.Fatal("expected err")
	}
}

func TestPodmanCreateRunArgs(t *testing.T) {
	fr := &fakeRunner{t: t}
	p := NewPodman("img:1", fr)
	p.PodmanBin = "podman"
	id, err := p.Create(context.Background(), sandboxclient.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fr.commands) != 1 {
		t.Fatalf("commands=%v", fr.commands)
	}
	run := fr.commands[0]
	if !strings.Contains(strings.Join(run, " "), "run") {
		t.Fatalf("expected run command, got %v", fr.commands)
	}
	name := p.containerName(id)
	want := strings.Join([]string{"podman", "run", "-d", "--pull", "never", "--name", name, "img:1", "sleep", "infinity"}, " ")
	got := strings.Join(run, " ")
	if got != want {
		t.Fatalf("\nwant %s\ngot  %s", want, got)
	}
}
