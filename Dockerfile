# Polyglot sandbox for autonomous dependency-upgrade agents.
# Runtimes and build tools via https://mise.jdx.dev — see docker/sandbox-tools.*.toml.
#
# On-demand versions (slim default), e.g.:
#   mise exec go@1.25.8 -- go test ./...
#   mise exec java@temurin-21.0.6 -- mvn -q test
#
# Build (requires network to download toolchains):
#   podman build -t sandbox-env:latest .                    # slim (~1.5–2.5 GB)
#   podman build --build-arg SANDBOX_PROFILE=full -t sandbox-env:full .
# Multi-arch:
#   podman build --platform linux/amd64,linux/arm64 -t sandbox-env:latest .
FROM debian:bookworm-slim

# minimal = one version per language; full = multi-version bake (see docker/sandbox-tools.*.toml)
ARG SANDBOX_PROFILE=minimal

ENV MISE_INSTALL_PATH=/usr/local/bin/mise
ENV MISE_DATA_DIR=/opt/mise
ENV MISE_CONFIG_DIR=/etc/mise
ENV MISE_TRUSTED_CONFIG_PATHS=/etc/mise:/workspace
ENV PATH="/opt/mise/shims:/usr/local/bin:/usr/bin:/bin"
ENV SANDBOX_PROFILE=${SANDBOX_PROFILE}

# Layer 1: OS packages + mise installer (changes rarely).
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      bash \
      build-essential \
      ca-certificates \
      curl \
      g++ \
      gcc \
      git \
      jq \
      libssl-dev \
      make \
      openssh-client \
      pkg-config \
      rsync \
      tar \
      unzip \
      wget \
      zip \
    ; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*; \
    curl -fsSL https://mise.run | sh; \
    mise --version

# Layer 2: mise configuration (bust cache when versions or profile change).
COPY docker/sandbox-tools.minimal.toml docker/sandbox-tools.full.toml /docker/
COPY docker/mise-settings.toml /etc/mise/settings.toml
RUN cp "/docker/sandbox-tools.${SANDBOX_PROFILE}.toml" /etc/mise/config.toml

# Layer 3: install toolchains and set global defaults.
RUN set -eux; \
    mise install; \
    mise use -g \
      go@1.26.3 \
      java@temurin-26.0.0 \
      node@24.16.0 \
      python@3.12.10 \
      maven@3.9.9 \
      gradle@9.5.1 \
      uv@latest \
      pnpm@10.11.0 \
      yarn@4.9.2; \
    python -m pip install 'poetry>=2.4,<3'; \
    mise reshim; \
    python -m ensurepip --upgrade

# Layer 4: build-time smoke checks (defaults on PATH).
RUN set -eux; \
    go version; \
    java -version; \
    node -v; \
    npm -v; \
    python -V; \
    python -m pip --version; \
    mvn -version; \
    gradle -version; \
    uv --version; \
    poetry --version; \
    pnpm --version; \
    yarn --version; \
    git --version; \
    jq --version

# Full profile only: verify additional pre-installed versions.
RUN set -eux; \
    if [ "$SANDBOX_PROFILE" = "full" ]; then \
      mise exec gradle@8.14.3 -- gradle -version; \
      mise exec go@1.24.11 -- go version; \
      mise exec go@1.25.8 -- go version; \
      mise exec node@22.22.3 -- node -v; \
      mise exec node@26.3.0 -- node -v; \
      mise exec python@3.11.13 -- python -V; \
      mise exec python@3.13.3 -- python -V; \
      mise exec java@temurin-21.0.6 -- java -version; \
      for py in 3.11.13 3.13.3; do \
        mise exec python@${py} -- python -m ensurepip --upgrade; \
      done; \
    fi

# Login shells (e.g. podman exec /bin/sh -lc) source /etc/profile and drop image ENV.
RUN set -eux; \
    printf '%s\n' \
      'export PATH="/opt/mise/shims:/usr/local/bin:/usr/bin:/bin"' \
      'export MISE_DATA_DIR="/opt/mise"' \
      'export MISE_CONFIG_DIR="/etc/mise"' \
      'export MISE_TRUSTED_CONFIG_PATHS="/etc/mise:/workspace"' \
      > /etc/profile.d/sandbox-env.sh

WORKDIR /workspace

CMD ["sleep", "infinity"]
