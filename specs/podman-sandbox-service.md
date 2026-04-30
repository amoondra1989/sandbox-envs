---
title: Local Podman sandbox service (Daytona-like)
description: Go REST service + client wrapping Podman CLI for create/exec/files/health sandbox lifecycle.
status: implemented
author: Abhishek Moondra <abhishek.moondra@sixt.com>
---

# Feature: podman-sandbox-service

## Goal

Single-process local sandbox API for experimentation: provision long-running containers via Podman CLI, run sync exec and file IO, destroy on demand. No Daytona/cloud dependency.

## Acceptance Criteria

- [x] Core package exposes `sandbox` interface matching agreed signatures: `Create`, `Destroy`, `Exec`, `ReadFile`, `WriteFile`, `HealthCheck`; `CreateOptions` / `ExecOptions` minimal (timeouts, cwd, env optional where trivial).
- [x] HTTP server maps JSON REST endpoints to those operations; bind configurable (default `127.0.0.1`); **no auth** (document localhost-only expectation).
- [x] Backend uses **fixed** base image (Dockerfile or pinned pull image) with Python, curl, wget, git; image ID/name from server env/config; sandbox registry **in-memory only** (restart invalidates IDs).
- [x] `pkg/` Go **HTTP client** + **exported request/response types** (downstream-importable); **no** dependency from `pkg/` on `internal/` — consumers must only import `pkg/...`.
- [x] Root **`go.mod` `module`**: `github.com/amoondra1989/sandbox-envs`; README documents **`go get github.com/amoondra1989/sandbox-envs/pkg/sandboxclient`** + minimal import example (adjust subpath if package name differs).
- [x] README rewritten: prerequisites (Go, Podman, macOS/Linux notes), build/run server + client smoke flow, REST route table, security caveat (no auth), **using the client from another Go module** (replace directive optional for local dev).

## Approach

Implement `internal/sandbox.PodmanSandbox` spawning `podman run -d` with fixed image, storing `sandboxID -> container name/ID`. `Exec` via `podman exec`; files via `podman cp` or exec cat/redirect; `Destroy` via `podman rm -f`; `HealthCheck` via `podman inspect` + running state. Wrap subprocess calls with timeouts and structured errors; parse stdout/stderr/exit code into `ExecResult`.

REST: thin handlers validating paths/body; JSON shapes match **exported** `pkg` types so server handlers encode/decode same structs **copied or shared via pkg-only imports** (handlers import pkg types; domain adapter maps pkg DTOs ↔ internal sandbox calls). Client library: prefer **stdlib** `net/http` + JSON to keep downstream graphs light.

Server starts container workspace suitable for Python/scripts and outbound network using Podman default bridge unless constrained later.

## Affected Modules

- `cmd/server` — HTTP listen, wiring sandbox impl + config (listen addr, image ref).
- `internal/sandbox` — interfaces, types (`CreateOptions`, `ExecOptions`, `ExecResult`), Podman CLI adapter.
- `internal/api` or `internal/handlers` — REST routing only (boundary between HTTP and domain).
- `pkg/sandboxclient` (name TBD) — typed REST client + DTOs for **external** Go modules (`go get github.com/amoondra1989/sandbox-envs/pkg/sandboxclient`).
- `Dockerfile` or `images/` — fixed sandbox runtime image definition.
- `README.md` — setup and usage.

## Test Strategy

- `go test ./...`: unit tests for handler validation and sandbox adapter with **fake/command recorder** or build-tag integration tests skipped by default; optional `-tags=integration` locally invoking real Podman if CI lacks it.
- Manual: `podman` installed → build server → `Create` → `Exec` python/curl/git smoke → `Destroy`.

## Out of Scope

- Persisted sandbox registry across process restart.
- Caller-selectable images, auth, TLS, multi-tenant quotas.
- Streaming/async exec, WebSockets.
- Kubernetes, rootless Podman edge cases beyond documenting host requirements.

## Notes

- Prefer rootless Podman where supported; document `podman machine start` on macOS if needed.
- Clarified stack: Go server + Go client only; REST JSON.
- **Downstream repos**: recommend **semver git tags** (`v0.x`) so other projects pin releases; breaking API changes bump minor/major per usual Go module rules.
- **Canonical repo**: [github.com/amoondra1989](https://github.com/amoondra1989); intended module **`github.com/amoondra1989/sandbox-envs`** once this workspace is pushed under that repo name (rename repo/module if GitHub repo slug differs).

### Implementation notes (done)

- Wire DTOs live in **`pkg/sandboxclient`**; **`internal/sandbox.Sandbox`** uses those types so handlers decode the same structs the client encodes (`internal` may import `pkg`, not vice versa).
- **Listen default** `127.0.0.1:8080` via `SANDBOX_LISTEN`.
- **Exec** uses `/bin/sh -lc`; **`ExecOptions.TimeoutSeconds`** wraps `context.WithTimeout` server-side.
- **ReadFile** uses `podman exec … cat -- <path>`; **WriteFile** uses temp file + `podman cp`.
- **`internal/api/integration_client_test.go`** is tagged **`integration`** and drives **`pkg/sandboxclient`** against **`httptest.Server`** + real Podman; requires **`SANDBOX_INTEGRATION=1`** (see README).
