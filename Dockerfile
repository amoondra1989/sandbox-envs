# Runtime image for sandboxes (Python, curl, wget, git). Build and tag locally, e.g.:
#   podman build -t sandbox-env:latest .
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    python3 \
    python3-venv \
    wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

CMD ["sleep", "infinity"]
