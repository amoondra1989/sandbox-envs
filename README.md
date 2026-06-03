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

From the repo root (downloads toolchains via [mise](https://mise.jdx.dev); needs network):

```bash
# Slim (default): one runtime per language — smaller image; other versions via mise at runtime
podman build -t sandbox-env:latest .

# Full: multiple Go/Java/Node/Python versions pre-installed — faster, ~4 GB (vs ~1.8 GB slim on arm64)
podman build --build-arg SANDBOX_PROFILE=full -t sandbox-env:full .
```

Versions live in [`docker/sandbox-tools.minimal.toml`](docker/sandbox-tools.minimal.toml) and [`docker/sandbox-tools.full.toml`](docker/sandbox-tools.full.toml). See [`docs/sandbox-image.md`](docs/sandbox-image.md) for profiles, on-demand install, and integration tests.

Multi-arch (optional): `podman build --platform linux/amd64,linux/arm64 -t sandbox-env:latest .`

### 2. (Optional) Override image or Podman binary

| Env var | Purpose |
|---------|---------|
| `SANDBOX_IMAGE` | Image used for `podman run` (default `sandbox-env:latest`; uses **`--pull never`** so the image must exist locally) |
| `SANDBOX_PODMAN_BIN` | Podman executable (default `podman`) |
| `SANDBOX_LISTEN` | Listen address (default `127.0.0.1:8080`) |
| `SANDBOX_CONTAINER_SOCKET` | Host path for the socket bind-mount (optional). On **macOS**, leave unset: the server uses the in-VM path from **`podman info`** (e.g. `/run/user/UID/podman/podman.sock`). Do **not** set this to the Podman Machine **`-api.sock`** path from `podman machine inspect` — it cannot be mounted into containers. |
| `SANDBOX_SOCKET_PRIVILEGED` | When socket mount is on, add **`--privileged`** to sandbox containers (default **true**). Set **`false`** only for rootful Podman where tests work without it. |
| `SANDBOX_TESTCONTAINERS_HOST_OVERRIDE` | Hostname for published container ports when tests run inside the sandbox (default **`host.containers.internal`**). |

### Container socket mount (default on)

By default, every new sandbox mounts the host Podman/Docker API socket at **`/var/run/docker.sock`** and sets **`DOCKER_HOST`**, **`TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE`**, and **`TESTCONTAINERS_HOST_OVERRIDE=host.containers.internal`** (plus **`--add-host …:host-gateway`**) so Testcontainers can start sibling containers on the host and tests inside the sandbox can reach their published ports (not via `localhost`).

Disable when not needed:

```json
POST /v1/sandboxes
{"mount_container_socket": false}
```

On **macOS with Podman Machine**, the mount source is the **in-VM** socket from **`podman info`** (not the host `…-api.sock` proxy). Sandboxes with socket mount run **`--privileged`** by default so processes inside the sandbox can use `/var/run/docker.sock` (rootless Podman otherwise returns permission denied). Disable with `SANDBOX_SOCKET_PRIVILEGED=false` only if you use a **rootful** machine and have verified Testcontainers works without it.

Testcontainers also receives **`TESTCONTAINERS_RYUK_DISABLED=true`** (recommended for rootless Podman). After changing socket settings, **recreate sandboxes** (destroy old ones and provision again).

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
| `POST` | `/v1/sandboxes` | [`sandboxclient.CreateOptions`](pkg/sandboxclient/types.go) — **`mount_container_socket`** defaults **true**; set **false** to disable | **201** JSON `{"sandbox_id":"..."}` |
| `DELETE` | `/v1/sandboxes/{id}` | — | **204** |
| `POST` | `/v1/sandboxes/{id}/exec` | [`sandboxclient.ExecRequest`](pkg/sandboxclient/types.go) (`command`, optional `opts`) | **200** [`ExecResult`](pkg/sandboxclient/types.go) |

**Exec `opts`:** optional **`cwd`** sets the working directory for `command`. **`working_dir`** is a legacy alias; if both are non-empty, **`cwd` wins**. Omit both (or use empty strings) for backward-compatible behavior (image default, typically **`/workspace`**). Non-empty **`cwd`**: must be absolute, no **`..`**, and must exist as a directory inside the container → otherwise **400**.

| `GET` | `/v1/sandboxes/{id}/file?path=/abs/path` | — | **200** raw **`application/octet-stream`** |
| `PUT` | `/v1/sandboxes/{id}/file?path=/abs/path` | raw octets (`application/octet-stream`) | **204** |
| `GET` | `/v1/sandboxes/{id}/health` | — | **204** |

Path rules for file APIs: **`path` must be absolute** (`/`…) and must **not** contain `..`.

Errors return JSON `{"error":"..."}` with appropriate status (e.g. **404** unknown sandbox, **400** bad path/body/**cwd**, **503** unhealthy).

---

## Capabilities inside the sandbox

The image is **`debian:bookworm-slim`** plus **[mise](https://mise.jdx.dev)** for polyglot runtimes and build tools. Full design: [`docs/sandbox-image.md`](docs/sandbox-image.md).

**Default image (`sandbox-env:latest`, slim profile)** — one version per language on `PATH`:

Go **1.26.3**, Temurin **26**, Node **24.16.0**, Python **3.12.10**, Maven **3.9.9**, Gradle **9.5.1**, **uv**, **poetry** (pip), **pnpm**, **yarn**, **npm**.

Other versions are **not** baked in; install on demand (needs network, cached under `/opt/mise` for the life of the sandbox):

```bash
mise exec go@1.25.8 -- go test ./...
mise exec java@temurin-21.0.6 -- mvn -q test
mise install node@22.22.3
```

After clone, agents can run **`mise install`** in the repo when a `.mise.toml` exists.

**Full profile (`sandbox-env:full`)** — also preinstalls Go 1.24/1.25, Java 21/24/25, Node 22/26, Python 3.11/3.13, Gradle 8.14.3 (larger image, no download for those versions).

**System packages:** git, curl, wget, jq, unzip, zip, make, gcc, g++, build-essential, openssh-client, rsync, tar, bash, CA certs, pkg-config, libssl-dev.

**Integration tests:** host socket at `/var/run/docker.sock` (default on create). See [Container socket mount](#container-socket-mount-default-on) and `docs/sandbox-image.md`.

Rebuild after changing [`Dockerfile`](Dockerfile) or `docker/sandbox-tools.*.toml`.

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

	res, err := c.Exec(ctx, id, `python -c "print(1+1)"`, sandboxclient.ExecOptions{})
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
  -d '{"command":"python -c \"print(\\\"hi\\\")\"","opts":{}}'
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
| `docker/` | mise config (`sandbox-tools.minimal.toml`, `sandbox-tools.full.toml`, `mise-settings.toml`) |
| `docs/sandbox-image.md` | Agent image design, versions, integration tests |
| `internal/api` | REST handlers |
| `internal/sandbox` | Domain + Podman CLI adapter |
| `pkg/sandboxclient` | **Stable import path** — HTTP client + JSON DTOs |
| `specs/sandbox-agent-image.md` | Image feature spec |

`pkg/` must not import `internal/` so downstream modules only depend on the published surface.
