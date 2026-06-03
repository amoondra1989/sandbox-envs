---
title: Autonomous dependency-upgrade sandbox image
description: Polyglot mise-based sandbox image for Java, Go, Python, and Node.js dependency-upgrade agents.
status: implemented
author: Abhishek Moondra <abhishek.moondra@sixt.com>
---

# Feature: sandbox-agent-image

## Goal

Provide a reproducible, fast-starting container image (`sandbox-env:latest`) that supports autonomous agents cloning repositories, switching runtimes, upgrading dependencies, building, running unit and integration tests, and producing commits—across Java, Go, Python, and Node.js with minimal per-repo setup.

## Acceptance Criteria

- [x] Image built from [`Dockerfile`](../Dockerfile) + `docker/sandbox-tools.{minimal,full}.toml` using **mise** as the sole runtime manager.
- [x] **Minimal profile (default):** one version per language on PATH; other versions via `mise exec` / `mise install` at runtime.
- [x] **Full profile:** optional multi-version bake (`SANDBOX_PROFILE=full`) for faster cold path without downloads.
- [x] Defaults on PATH: Go **1.26.3**, Java **26**, Node **24**, Python **3.12**, Maven **3.9.9**, Gradle **9.5.1**, **uv**, **poetry** (pip), **pnpm**, **yarn**.
- [x] APT packages include git, curl, wget, jq, unzip, zip, make, gcc, g++, build-essential, openssh-client, ca-certificates, tar, bash, rsync, pkg-config, libssl-dev.
- [x] [`docs/sandbox-image.md`](../docs/sandbox-image.md) documents version rationale, integration-test socket strategy, security/ops tradeoffs, and size estimate.
- [x] [`README.md`](../README.md) capabilities section updated to match the image matrix.
- [x] Integration tests continue to use **host socket mount** (server default); no Docker-in-Docker in the image.

## Version matrix

**Minimal profile — baked:** Go 1.26.3, Java 26, Node 24, Python 3.12, Maven 3.9.9, Gradle 9.5.1, uv, pnpm, yarn.

**Full profile — additionally baked:** Go 1.24/1.25, Java 21/24/25, Node 22/26, Python 3.11/3.13, Gradle 8.14.3.

Refresh by editing `docker/sandbox-tools.minimal.toml` / `docker/sandbox-tools.full.toml` and rebuilding.

## Approach

Expand the existing Debian + mise image: single APT layer, copy TOML config, `mise install` + global `mise use`, enable corepack for yarn, verify toolchains in build. Document integration-test support via existing `mount_container_socket` server behavior (no image-level DinD).

## Affected Modules

- `Dockerfile` — APT tooling, mise install/verify, corepack.
- `docker/sandbox-tools.minimal.toml`, `docker/sandbox-tools.full.toml` — profile tool lists.
- `docker/mise-settings.toml` — optional mise settings (trusted paths).
- `docs/sandbox-image.md` — design and operations doc.
- `README.md` — user-facing capabilities and build notes.

## Test Strategy

- `podman build -t sandbox-env:latest .` succeeds.
- In-container smoke: `go version`, `node -v`, `python -V`, `java -version`, `mvn -version`, `gradle -version`, `uv --version`, `poetry --version`, `pnpm -version`, `yarn -v`.
- `go test ./...` for the Go server (unchanged).
- Optional: `SANDBOX_INTEGRATION=1 go test -tags=integration` after image rebuild.

## Out of Scope

- Docker-in-Docker or Podman inside the sandbox image.
- CI publish pipeline for multi-arch images.
- `docker compose` CLI in the image (follow-up if needed).
- managed-agents code changes.

## Notes

- macOS hosts: sandbox server uses in-VM Podman socket for bind mounts; see README.
- Java 11/17 removed from bake; install on demand via `mise install java@temurin-17.0.14`.
