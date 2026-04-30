package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/amoondra1989/sandbox-envs/pkg/sandboxclient"
)

// PodmanSandbox manages containers via the podman CLI.
type PodmanSandbox struct {
	Image      string
	PodmanBin  string
	RunCmd     Runner
	mu         sync.RWMutex
	containers map[string]string // sandboxID -> container name
}

func NewPodman(image string, run Runner) *PodmanSandbox {
	if image == "" {
		image = os.Getenv("SANDBOX_IMAGE")
	}
	if image == "" {
		// Avoid "localhost/foo" tags: Podman treats them as a registry host and probes https://localhost/v2/.
		image = "sandbox-env:latest"
	}
	bin := os.Getenv("SANDBOX_PODMAN_BIN")
	if bin == "" {
		bin = "podman"
	}
	if run == nil {
		run = ExecRunner{}
	}
	return &PodmanSandbox{
		Image:      image,
		PodmanBin:  bin,
		RunCmd:     run,
		containers: make(map[string]string),
	}
}

func (p *PodmanSandbox) containerName(id string) string {
	return "sandbox-" + id
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func validateSandboxPath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("path is empty")
	}
	if !strings.HasPrefix(filePath, "/") {
		return fmt.Errorf("path must be absolute")
	}
	if strings.Contains(filePath, "..") {
		return fmt.Errorf("invalid path")
	}
	return nil
}

func (p *PodmanSandbox) lookup(id string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	name, ok := p.containers[id]
	if !ok {
		return "", ErrNotFound
	}
	return name, nil
}

func (p *PodmanSandbox) register(id, name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.containers[id] = name
}

func (p *PodmanSandbox) unregister(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.containers, id)
}

// Create implements Sandbox.
func (p *PodmanSandbox) Create(ctx context.Context, _ sandboxclient.CreateOptions) (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}
	name := p.containerName(id)
	args := []string{
		"run", "-d",
		"--pull", "never",
		"--name", name,
		p.Image,
		"sleep", "infinity",
	}
	stdout, stderr, code, err := p.RunCmd.Run(ctx, p.PodmanBin, args...)
	if err != nil {
		return "", fmt.Errorf("podman run: %w (stderr=%q)", err, string(stderr))
	}
	if code != 0 {
		return "", fmt.Errorf("podman run: exit %d stdout=%q stderr=%q", code, string(stdout), string(stderr))
	}
	p.register(id, name)
	_ = stdout // container ID — unused; name is authoritative
	return id, nil
}

// Destroy implements Sandbox.
func (p *PodmanSandbox) Destroy(ctx context.Context, sandboxID string) error {
	name, err := p.lookup(sandboxID)
	if err != nil {
		return err
	}
	stdout, stderr, code, runErr := p.RunCmd.Run(ctx, p.PodmanBin, "rm", "-f", name)
	if runErr != nil {
		return fmt.Errorf("podman rm: %w (stderr=%q)", runErr, string(stderr))
	}
	if code != 0 {
		return fmt.Errorf("podman rm: exit %d stdout=%q stderr=%q", code, string(stdout), string(stderr))
	}
	p.unregister(sandboxID)
	return nil
}

// Exec implements Sandbox.
func (p *PodmanSandbox) Exec(ctx context.Context, sandboxID string, command string, opts sandboxclient.ExecOptions) (*sandboxclient.ExecResult, error) {
	name, err := p.lookup(sandboxID)
	if err != nil {
		return nil, err
	}
	wd := EffectiveWorkingDir(opts)
	if err := ValidateExecWorkingDirSyntax(wd); err != nil {
		return nil, err
	}
	if wd != "" {
		if err := p.verifyExecWorkingDir(ctx, name, wd); err != nil {
			return nil, err
		}
	}
	if opts.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.TimeoutSeconds)*time.Second)
		defer cancel()
	}
	args := []string{"exec"}
	if wd != "" {
		args = append(args, "--workdir", wd)
	}
	for k, v := range opts.Env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, name, "/bin/sh", "-lc", command)

	stdout, stderr, code, runErr := p.RunCmd.Run(ctx, p.PodmanBin, args...)
	if runErr != nil {
		return nil, fmt.Errorf("podman exec: %w (stderr=%q)", runErr, string(stderr))
	}
	return &sandboxclient.ExecResult{
		ExitCode: code,
		Stdout:   append([]byte(nil), stdout...),
		Stderr:   append([]byte(nil), stderr...),
	}, nil
}

func (p *PodmanSandbox) verifyExecWorkingDir(ctx context.Context, containerName, dir string) error {
	_, stderr, code, runErr := p.RunCmd.Run(ctx, p.PodmanBin, "exec", containerName, "test", "-d", dir)
	if runErr != nil {
		return fmt.Errorf("%w: verify working directory: %v", ErrInvalidExecWorkingDir, runErr)
	}
	if code != 0 {
		msg := strings.TrimSpace(string(stderr))
		if msg != "" {
			return fmt.Errorf("%w: working directory does not exist or is not a directory: %s (%s)", ErrInvalidExecWorkingDir, dir, msg)
		}
		return fmt.Errorf("%w: working directory does not exist or is not a directory: %s", ErrInvalidExecWorkingDir, dir)
	}
	return nil
}

// ReadFile implements Sandbox.
func (p *PodmanSandbox) ReadFile(ctx context.Context, sandboxID string, filePath string) ([]byte, error) {
	if err := validateSandboxPath(filePath); err != nil {
		return nil, err
	}
	name, err := p.lookup(sandboxID)
	if err != nil {
		return nil, err
	}
	stdout, stderr, code, runErr := p.RunCmd.Run(ctx, p.PodmanBin, "exec", name, "cat", "--", filePath)
	if runErr != nil {
		return nil, fmt.Errorf("podman exec cat: %w (stderr=%q)", runErr, string(stderr))
	}
	if code != 0 {
		return nil, fmt.Errorf("read file: exit %d: %s", code, string(stderr))
	}
	return append([]byte(nil), stdout...), nil
}

// WriteFile implements Sandbox.
func (p *PodmanSandbox) WriteFile(ctx context.Context, sandboxID string, filePath string, content []byte) error {
	if err := validateSandboxPath(filePath); err != nil {
		return err
	}
	name, err := p.lookup(sandboxID)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp("", "sandbox-write-*")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	dest := name + ":" + filePath
	stdout, stderr, code, runErr := p.RunCmd.Run(ctx, p.PodmanBin, "cp", tmpPath, dest)
	if runErr != nil {
		return fmt.Errorf("podman cp: %w (stderr=%q)", runErr, string(stderr))
	}
	if code != 0 {
		return fmt.Errorf("podman cp: exit %d stdout=%q stderr=%q", code, string(stdout), string(stderr))
	}
	return nil
}

// HealthCheck implements Sandbox.
func (p *PodmanSandbox) HealthCheck(ctx context.Context, sandboxID string) error {
	name, err := p.lookup(sandboxID)
	if err != nil {
		return err
	}
	stdout, stderr, code, runErr := p.RunCmd.Run(ctx, p.PodmanBin,
		"inspect", "-f", "{{.State.Running}}", name)
	if runErr != nil {
		return fmt.Errorf("podman inspect: %w (stderr=%q)", runErr, string(stderr))
	}
	if code != 0 {
		return fmt.Errorf("podman inspect: exit %d stdout=%q stderr=%q", code, string(stdout), string(stderr))
	}
	state := strings.TrimSpace(string(stdout))
	if state != "true" {
		return fmt.Errorf("sandbox not running (state=%q stderr=%q)", state, string(stderr))
	}
	return nil
}
