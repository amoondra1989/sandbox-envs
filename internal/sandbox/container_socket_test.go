package sandbox

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"testing"
)

type socketProbeRunner struct {
	infoPath string
}

func (r *socketProbeRunner) Run(ctx context.Context, bin string, args ...string) ([]byte, []byte, int, error) {
	_ = ctx
	_ = bin
	if len(args) >= 2 && args[0] == "info" {
		return []byte(r.infoPath + "\n"), nil, 0, nil
	}
	return nil, []byte("unexpected: " + fmt.Sprint(args)), 1, nil
}

func TestValidateContainerSocket(t *testing.T) {
	t.Parallel()
	sock := fmt.Sprintf("/tmp/sandbox-envs-validate-%d.sock", os.Getpid())
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
		_ = os.Remove(sock)
	})
	if err := validateContainerSocket(sock); err != nil {
		t.Fatal(err)
	}
	if err := validateContainerSocket("/tmp/sandbox-envs-missing.sock"); err == nil {
		t.Fatal("expected error for missing socket")
	}
}

func TestValidateContainerSocketRejectsMachineAPI(t *testing.T) {
	t.Parallel()
	path := "/var/folders/xx/T/podman/podman-machine-default-api.sock"
	if err := validateContainerSocket(path); err == nil {
		t.Fatal("expected error for machine API socket")
	}
}

func TestValidateContainerSocketAllowsPodmanVMPathOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("VM socket path is only special-cased on darwin")
	}
	t.Parallel()
	if err := validateContainerSocket("/run/user/502/podman/podman.sock"); err != nil {
		t.Fatal(err)
	}
}

func TestContainerSocketRunArgs(t *testing.T) {
	t.Parallel()
	args := containerSocketRunArgs("/host/podman.sock")
	want := []string{
		"-v", "/host/podman.sock:/var/run/docker.sock:ro",
		"-e", "DOCKER_HOST=unix:///var/run/docker.sock",
		"-e", "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock",
	}
	if len(args) != len(want) {
		t.Fatalf("got %v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("index %d: got %q want %q", i, args[i], want[i])
		}
	}
}

func TestResolveContainerSocketUsesInfoPathOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin uses podman info VM socket for bind mounts")
	}
	t.Parallel()
	want := "/run/user/502/podman/podman.sock"
	p := &PodmanSandbox{
		PodmanBin: "podman",
		RunCmd: &socketProbeRunner{
			infoPath: "unix://" + want,
		},
	}
	got, err := p.resolveContainerSocket(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveContainerSocketExplicit(t *testing.T) {
	t.Parallel()
	sock := fmt.Sprintf("/tmp/sandbox-envs-explicit-%d.sock", os.Getpid())
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
		_ = os.Remove(sock)
	})

	p := &PodmanSandbox{
		ContainerSocket: sock,
		RunCmd:          &socketProbeRunner{infoPath: "/should/not/be/called"},
	}
	got, err := p.resolveContainerSocket(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != sock {
		t.Fatalf("got %q want %q", got, sock)
	}
}
