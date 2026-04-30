# sandbox-envs

Small **local sandbox HTTP API** (Daytona-like surface area) backed by **Podman**. Intended for experimentation on your machine: provision long-lived containers, run synchronous exec, read/write files, health-check, destroy.

**Security:** the HTTP API has **no authentication**. Bind to loopback (`127.0.0.1` by default) and do not expose this service on untrusted networks.

**Module:** `github.com/amoondra1989/sandbox-envs`  
**Public client (for other Go repos):** `github.com/amoondra1989/sandbox-envs/pkg/sandboxclient`

---

## Prerequisites

| Requirement | Notes |
|-------------|--------|
| **Go** | 1.22+ |
| **Podman** | `podman` on `PATH`; macOS: install Podman Desktop / CLI and run **`podman machine start`** before first use |
| **Sandbox image** | Default image tag: **`sandbox-env:latest`** — build locally from this repo’s `Dockerfile` (**do not** use `localhost/...` names: Podman treats them as a remote registry at `localhost`) |

Sandbox IDs and container mappings live **only in process memory**. Restarting the server invalidates existing IDs.

---

## Setup

### 1. Build the sandbox runtime image

From the repo root:

```bash
podman build -t sandbox-env:latest .
```

### 2. (Optional) Override image or Podman binary

| Env var | Purpose |
|---------|---------|
| `SANDBOX_IMAGE` | Image used for `podman run` (default `sandbox-env:latest`; uses **`--pull never`** so the image must exist locally) |
| `SANDBOX_PODMAN_BIN` | Podman executable (default `podman`) |
| `SANDBOX_LISTEN` | Listen address (default `127.0.0.1:8080`) |

---

## Run the server

```bash
go run ./cmd/server
```

Logs show the listen URL (default `http://127.0.0.1:8080`).

---

## REST API

Base path: **`/v1`**. Request/response bodies use JSON unless noted.

| Method | Path | Body | Success |
|--------|------|------|---------|
| `POST` | `/v1/sandboxes` | `{ }` — [`sandboxclient.CreateOptions`](pkg/sandboxclient/types.go) | **201** JSON `{"sandbox_id":"..."}` |
| `DELETE` | `/v1/sandboxes/{id}` | — | **204** |
| `POST` | `/v1/sandboxes/{id}/exec` | [`sandboxclient.ExecRequest`](pkg/sandboxclient/types.go) (`command`, optional `opts`) | **200** [`ExecResult`](pkg/sandboxclient/types.go) |
| `GET` | `/v1/sandboxes/{id}/file?path=/abs/path` | — | **200** raw **`application/octet-stream`** |
| `PUT` | `/v1/sandboxes/{id}/file?path=/abs/path` | raw octets (`application/octet-stream`) | **204** |
| `GET` | `/v1/sandboxes/{id}/health` | — | **204** |

Path rules for file APIs: **`path` must be absolute** (`/`…) and must **not** contain `..`.

Errors return JSON `{"error":"..."}` with appropriate status (e.g. **404** unknown sandbox, **400** bad path/body, **503** unhealthy).

---

## Capabilities inside the sandbox

The stock `Dockerfile` installs **Python 3**, **curl**, **wget**, **git**, and CA certs so typical scripts and outbound HTTPS/Git operations work over Podman’s default networking.

---

## Go client (this repo or another project)

### Import from GitHub

After pushing public tags (recommended: **`v0.x.y`** semver):

```bash
go get github.com/amoondra1989/sandbox-envs/pkg/sandboxclient@v0.1.0
```

Example:

```go
package main

import (
	"context"
	"fmt"

	"github.com/amoondra1989/sandbox-envs/pkg/sandboxclient"
)

func main() {
	c := sandboxclient.Client{BaseURL: "http://127.0.0.1:8080"}
	ctx := context.Background()
	id, err := c.Create(ctx, sandboxclient.CreateOptions{})
	if err != nil {
		panic(err)
	}
	defer func() { _ = c.Destroy(ctx, id) }()

	res, err := c.Exec(ctx, id, `python3 -c "print(1+1)"`, sandboxclient.ExecOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Println(res.ExitCode, string(res.Stdout))
}
```

### Local development with `replace`

In the **consumer** repo’s `go.mod`:

```go
replace github.com/amoondra1989/sandbox-envs => ../path/to/sandbox-envs
```

Then `go get github.com/amoondra1989/sandbox-envs/pkg/sandboxclient` resolves to your checkout.

---

## Smoke test (manual)

With the server running and the image built:

```bash
curl -sS -X POST http://127.0.0.1:8080/v1/sandboxes -H 'Content-Type: application/json' -d '{}'
# use returned sandbox_id:
curl -sS -X POST http://127.0.0.1:8080/v1/sandboxes/$SID/exec \
  -H 'Content-Type: application/json' \
  -d '{"command":"python3 -c \"print(\\\"hi\\\")\"","opts":{}}'
curl -sS -X DELETE http://127.0.0.1:8080/v1/sandboxes/$SID
```

---

## Tests

```bash
go test ./...
```

**Integration** (real Podman + **`pkg/sandboxclient`** over HTTP): build the sandbox image first, then:

```bash
export SANDBOX_INTEGRATION=1
go test -tags=integration -count=1 -timeout=5m ./internal/api -run Integration
```

Without `SANDBOX_INTEGRATION=1`, that test **skips**. Without `-tags=integration`, the file is not compiled (default CI stays fast).

---

## Layout

| Path | Role |
|------|------|
| `cmd/server` | HTTP server entrypoint |
| `internal/api` | REST handlers |
| `internal/sandbox` | Domain + Podman CLI adapter |
| `pkg/sandboxclient` | **Stable import path** — HTTP client + JSON DTOs |

`pkg/` must not import `internal/` so downstream modules only depend on the published surface.
