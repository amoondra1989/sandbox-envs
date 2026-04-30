//go:build integration

package api_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"net/http/httptest"

	"github.com/amoondra1989/sandbox-envs/internal/api"
	"github.com/amoondra1989/sandbox-envs/internal/sandbox"
	"github.com/amoondra1989/sandbox-envs/pkg/sandboxclient"
)

// TestIntegration_ClientPodman exercises pkg/sandboxclient against the real HTTP stack and Podman.
// Requires: podman on PATH, sandbox image built (default sandbox-env:latest). Enable with:
//
//	SANDBOX_INTEGRATION=1 go test -tags=integration -count=1 -timeout=5m ./internal/api -run Integration
func TestIntegration_ClientPodman(t *testing.T) {
	if os.Getenv("SANDBOX_INTEGRATION") != "1" {
		t.Skip("set SANDBOX_INTEGRATION=1 to run Podman + client integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	backend := sandbox.NewPodman("", nil)
	srv := httptest.NewServer(api.NewServer(backend))
	defer srv.Close()

	client := sandboxclient.Client{
		BaseURL: srv.URL,
		HTTP:    srv.Client(),
	}

	id, err := client.Create(ctx, sandboxclient.CreateOptions{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Logf("sandbox_id=%s", id)

	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer ccancel()
		_ = client.Destroy(cctx, id)
	})

	if err := client.HealthCheck(ctx, id); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}

	script := []byte("print(\"integration\")\n")
	const path = "/workspace/integration_client_test.py"
	if err := client.WriteFile(ctx, id, path, script); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	gotFile, err := client.ReadFile(ctx, id, path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(gotFile, script) {
		t.Fatalf("ReadFile content mismatch:\nwant %q\ngot  %q", script, gotFile)
	}

	res, err := client.Exec(ctx, id, "python3 integration_client_test.py", sandboxclient.ExecOptions{Cwd: "/workspace"})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("Exec exit=%d stderr=%q stdout=%q", res.ExitCode, res.Stderr, res.Stdout)
	}
	if string(bytes.TrimSpace(res.Stdout)) != "integration" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}

	if err := client.Destroy(ctx, id); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}
