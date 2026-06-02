package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amoondra1989/sandbox-envs/pkg/sandboxclient"
)

type fakeRunner struct {
	t         *testing.T
	commands  [][]string
	failTestD map[string]bool // paths where test -d should fail (exit 1)
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
		if len(args) >= 5 && args[2] == "test" && args[3] == "-d" {
			dir := args[4]
			if f.failTestD != nil && f.failTestD[dir] {
				return nil, []byte("not a directory"), 1, nil
			}
			return nil, nil, 0, nil
		}
		return []byte("out"), nil, 0, nil
	case "cp":
		return nil, nil, 0, nil
	case "inspect":
		return []byte("true\n"), nil, 0, nil
	case "info":
		return []byte("unix:///tmp/detected-podman.sock\n"), nil, 0, nil
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
	id, err := p.Create(ctx, sandboxclient.CreateOptions{MountContainerSocket: sandboxclient.Bool(false)})
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

func TestPodmanExecWorkingDirProbeAndFlag(t *testing.T) {
	fr := &fakeRunner{t: t}
	p := NewPodman("img:1", fr)
	p.PodmanBin = "podman"
	id, err := p.Create(context.Background(), sandboxclient.CreateOptions{MountContainerSocket: sandboxclient.Bool(false)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(context.Background(), id, "true", sandboxclient.ExecOptions{Cwd: "/workspace"}); err != nil {
		t.Fatal(err)
	}
	var sawProbe, sawMain bool
	for _, c := range fr.commands {
		s := strings.Join(c, " ")
		if strings.Contains(s, "test -d /workspace") {
			sawProbe = true
		}
		if strings.Contains(s, "--workdir") && strings.Contains(s, "/workspace") {
			sawMain = true
		}
	}
	if !sawProbe || !sawMain {
		t.Fatalf("probe=%v main=%v commands=%v", sawProbe, sawMain, fr.commands)
	}
}

func TestPodmanExecWorkingDirRejectMissingDir(t *testing.T) {
	fr := &fakeRunner{t: t, failTestD: map[string]bool{"/gone": true}}
	p := NewPodman("img:1", fr)
	p.PodmanBin = "podman"
	id, err := p.Create(context.Background(), sandboxclient.CreateOptions{MountContainerSocket: sandboxclient.Bool(false)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(context.Background(), id, "true", sandboxclient.ExecOptions{Cwd: "/gone"})
	if err == nil || !errors.Is(err, ErrInvalidExecWorkingDir) {
		t.Fatalf("want ErrInvalidExecWorkingDir got %v", err)
	}
}

func TestPodmanCreateWithContainerSocket(t *testing.T) {
	fr := &fakeRunner{t: t}
	p := NewPodman("img:1", fr)
	p.PodmanBin = "podman"
	sock := fmt.Sprintf("/tmp/sandbox-envs-%d.sock", os.Getpid())
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
		_ = os.Remove(sock)
	})
	p.ContainerSocket = sock

	id, err := p.Create(context.Background(), sandboxclient.CreateOptions{})
	if err != nil || id == "" {
		t.Fatalf("create: %v id=%q", err, id)
	}
	run := strings.Join(fr.commands[0], " ")
	if !strings.Contains(run, "-v "+sock+":/var/run/docker.sock:ro") {
		t.Fatalf("missing socket mount: %s", run)
	}
	if !strings.Contains(run, "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock") {
		t.Fatalf("missing container socket env: %s", run)
	}
}

func TestPodmanCreateContainerSocketRequiresSocket(t *testing.T) {
	fr := &fakeRunner{t: t}
	p := NewPodman("img:1", fr)
	p.PodmanBin = "podman"
	p.ContainerSocket = filepath.Join(t.TempDir(), "missing.sock")

	_, err := p.Create(context.Background(), sandboxclient.CreateOptions{})
	if err == nil || !errors.Is(err, ErrContainerSocketUnavailable) {
		t.Fatalf("want ErrContainerSocketUnavailable got %v", err)
	}
}

func TestPodmanCreateRunArgs(t *testing.T) {
	fr := &fakeRunner{t: t}
	p := NewPodman("img:1", fr)
	p.PodmanBin = "podman"
	id, err := p.Create(context.Background(), sandboxclient.CreateOptions{MountContainerSocket: sandboxclient.Bool(false)})
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
