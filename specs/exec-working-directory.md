---
title: Exec optional cwd (cwd + working_dir)
description: First-class optional working directory on POST exec with validation; backward-compatible JSON aliases.
status: implemented
author: Abhishek Moondra <abhishek.moondra@sixt.com>
---

# Feature: exec-working-directory

## Goal

Match downstream **`managed-agents`** contract: optional exec CWD via **`opts.cwd`**, validate paths, keep **`working_dir`** working; avoid fragile `cd &&` wrappers.

## Acceptance Criteria

- [x] JSON **`opts.cwd`** optional; empty/absent → same behavior as today (no `--workdir`).
- [x] JSON **`opts.working_dir`** still accepted (legacy); **`cwd` wins** if both non-empty after trim.
- [x] Non-empty cwd: absolute `/…`, reject `..` → **400** JSON error from API.
- [x] Non-empty cwd: must exist inside container as directory (`test -d`) → **400** if not (document in README).
- [x] **`pkg/sandboxclient.ExecOptions`**: **`Cwd`** (`json:"cwd"`) + **`WorkingDir`** (`json:"working_dir"`) documented.
- [x] README + integration exec exercise **`cwd`** where relevant.

## Approach

`EffectiveWorkingDir` merges `Cwd` / `WorkingDir`. Handlers validate syntax before `Exec`; `PodmanSandbox.Exec` re-validates syntax, runs `podman exec … test -d`, then `podman exec --workdir …` for main command.

## Affected Modules

- `pkg/sandboxclient/types.go` — `Cwd`, docs on `WorkingDir`.
- `internal/sandbox/exec_workdir.go` — helpers + `ErrInvalidExecWorkingDir`.
- `internal/sandbox/podman.go` — dir probe + `--workdir` from effective path.
- `internal/api/handlers.go` — map `ErrInvalidExecWorkingDir` → 400.
- `internal/api/*_test.go`, `internal/sandbox/podman_test.go` — cases.
- `internal/api/integration_client_test.go` — `cwd`.
- `README.md`

## Test Strategy

- Unit: syntax rejections via handler tests; fakeRunner extended for `test -d` probe vs main exec.
- `SANDBOX_INTEGRATION=1 go test -tags=integration ./internal/api -run Integration`.

## Out of Scope

- New timeout field naming; streaming exec.

## Notes

Downstream artifact: `MAINTAINER_REQUEST_EXEC_WORKING_DIRECTORY.md`.
