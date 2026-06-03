package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// ErrContainerSocketUnavailable means a container socket mount was requested but no runtime socket is available.
var ErrContainerSocketUnavailable = errors.New("container socket unavailable")

// In-container path where the host Podman/Docker API socket is mounted (Docker-compatible API).
const containerSocketMountTarget = "/var/run/docker.sock"

// Default hostname for published ports when tests run inside the sandbox (not on the host).
// Testcontainers starts sibling containers on the host; JDBC URLs must not use localhost.
const defaultTestcontainersHostOverride = "host.containers.internal"

func testcontainersHostOverride() string {
	if v := strings.TrimSpace(os.Getenv("SANDBOX_TESTCONTAINERS_HOST_OVERRIDE")); v != "" {
		return v
	}
	return defaultTestcontainersHostOverride
}

func containerSocketRunArgs(hostSocket string) []string {
	hostOverride := testcontainersHostOverride()
	var out []string
	if socketPrivilegedEnabled() {
		out = append(out, "--privileged")
	}
	// Reach host-published ports from inside the sandbox (Podman + Docker Desktop).
	out = append(out,
		"--add-host", "host.containers.internal:host-gateway",
		"--add-host", "host.docker.internal:host-gateway",
	)
	out = append(out,
		"-v", hostSocket+":"+containerSocketMountTarget+":ro",
		"-e", "DOCKER_HOST=unix://"+containerSocketMountTarget,
		"-e", "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="+containerSocketMountTarget,
		"-e", "TESTCONTAINERS_HOST_OVERRIDE="+hostOverride,
		// Rootless Podman: Ryuk often cannot run; required for Testcontainers with socket mount.
		"-e", "TESTCONTAINERS_RYUK_DISABLED=true",
	)
	return out
}

// socketPrivilegedEnabled reports whether sandboxes with a mounted socket should run privileged.
// Rootless Podman sockets are not usable from an unprivileged nested container (permission denied on
// /var/run/docker.sock). Set SANDBOX_SOCKET_PRIVILEGED=false when using a rootful machine/socket
// and you want to avoid --privileged.
func socketPrivilegedEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SANDBOX_SOCKET_PRIVILEGED")))
	if v == "0" || v == "false" || v == "no" {
		return false
	}
	return true
}

func normalizeSocketPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "unix://")
	return path
}

// isPodmanMachineAPISocket reports the macOS host proxy socket from podman machine inspect.
// It exists on the Mac filesystem but cannot be bind-mounted into containers (statfs: operation not supported).
func isPodmanMachineAPISocket(path string) bool {
	path = normalizeSocketPath(path)
	return strings.Contains(path, "/podman/") && strings.HasSuffix(path, "-api.sock")
}

// isPodmanVMSocketPath reports a socket path inside the Podman Machine VM (from podman info).
// It is not present on the macOS host but Podman accepts it as a volume source for podman run.
func isPodmanVMSocketPath(path string) bool {
	path = normalizeSocketPath(path)
	return strings.HasPrefix(path, "/run/") && strings.Contains(path, "/podman/")
}

func (p *PodmanSandbox) resolveContainerSocket(ctx context.Context) (string, error) {
	if s := normalizeSocketPath(p.ContainerSocket); s != "" {
		if err := validateContainerSocket(s); err != nil {
			return "", err
		}
		return s, nil
	}

	if runtime.GOOS == "darwin" {
		if host := os.Getenv("DOCKER_HOST"); strings.HasPrefix(host, "unix://") {
			path := normalizeSocketPath(host)
			if err := validateContainerSocket(path); err == nil {
				return path, nil
			}
		}
		return p.podmanInfoSocket(ctx)
	}

	candidates, err := p.containerSocketCandidates(ctx)
	if err != nil {
		return "", err
	}

	var reasons []string
	for _, path := range candidates {
		if err := validateContainerSocket(path); err == nil {
			return path, nil
		} else {
			reasons = append(reasons, err.Error())
		}
	}
	if len(reasons) == 0 {
		return "", fmt.Errorf("%w: no socket candidates", ErrContainerSocketUnavailable)
	}
	return "", fmt.Errorf("%w: %s", ErrContainerSocketUnavailable, strings.Join(reasons, "; "))
}

func (p *PodmanSandbox) containerSocketCandidates(ctx context.Context) ([]string, error) {
	var out []string
	seen := make(map[string]struct{})
	add := func(paths ...string) {
		for _, path := range paths {
			path = normalizeSocketPath(path)
			if path == "" {
				continue
			}
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			out = append(out, path)
		}
	}

	if host := os.Getenv("DOCKER_HOST"); strings.HasPrefix(host, "unix://") {
		add(host)
	}

	if path, err := p.podmanInfoSocket(ctx); err != nil {
		return nil, err
	} else {
		add(path)
	}

	if path, err := p.podmanMachineSocket(ctx); err == nil {
		add(path)
	}

	return out, nil
}

func (p *PodmanSandbox) podmanInfoSocket(ctx context.Context) (string, error) {
	stdout, stderr, code, err := p.RunCmd.Run(ctx, p.PodmanBin, "info", "-f", "{{.Host.RemoteSocket.Path}}")
	if err != nil {
		return "", fmt.Errorf("%w: podman info: %v (stderr=%q)", ErrContainerSocketUnavailable, err, string(stderr))
	}
	if code != 0 {
		return "", fmt.Errorf("%w: podman info exit %d: %s", ErrContainerSocketUnavailable, code, string(stderr))
	}
	path := normalizeSocketPath(string(stdout))
	if path == "" {
		return "", fmt.Errorf("%w: empty socket path from podman info", ErrContainerSocketUnavailable)
	}
	return path, nil
}

func (p *PodmanSandbox) podmanMachineSocket(ctx context.Context) (string, error) {
	stdout, stderr, code, err := p.RunCmd.Run(ctx, p.PodmanBin, "machine", "inspect", "--format", "{{.ConnectionInfo.PodmanSocket.Path}}")
	if err != nil {
		return "", fmt.Errorf("%w: podman machine inspect: %v (stderr=%q)", ErrContainerSocketUnavailable, err, string(stderr))
	}
	if code != 0 {
		return "", fmt.Errorf("%w: podman machine inspect exit %d: %s", ErrContainerSocketUnavailable, code, string(stderr))
	}
	path := normalizeSocketPath(string(stdout))
	if path == "" {
		return "", fmt.Errorf("%w: empty socket path from podman machine inspect", ErrContainerSocketUnavailable)
	}
	return path, nil
}

func validateContainerSocket(path string) error {
	path = normalizeSocketPath(path)
	if path == "" {
		return fmt.Errorf("%w: empty socket path", ErrContainerSocketUnavailable)
	}
	if isPodmanMachineAPISocket(path) {
		return fmt.Errorf("%w: %s is the macOS Podman Machine API proxy and cannot be bind-mounted; unset SANDBOX_CONTAINER_SOCKET to use the in-VM socket from podman info", ErrContainerSocketUnavailable, path)
	}
	if runtime.GOOS == "darwin" && isPodmanVMSocketPath(path) {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%w: socket %s: %v", ErrContainerSocketUnavailable, path, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%w: %s is not a unix socket", ErrContainerSocketUnavailable, path)
	}
	return nil
}
