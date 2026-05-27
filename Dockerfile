# Sandbox runtime: Go + Java (Maven/Gradle) via mise, plus Python/curl/wget/git.
#
# There is no small official "all JDKs + all Go versions" base image. This image uses
# https://mise.jdx.dev to install multiple toolchains and put shims on PATH.
#
# Switch versions inside a sandbox, e.g.:
#   mise exec go@1.22.12 -- go test ./...
#   mise exec java@temurin-17.0.14 -- mvn -q test
#
# Build (requires network to download toolchains):
#   podman build -t sandbox-env:latest .
FROM debian:bookworm-slim

ENV MISE_INSTALL_PATH=/usr/local/bin/mise
ENV MISE_DATA_DIR=/opt/mise
ENV MISE_CONFIG_DIR=/etc/mise
ENV MISE_TRUSTED_CONFIG_PATHS=/etc/mise
ENV PATH="/opt/mise/shims:/usr/local/bin:/usr/bin:/bin"

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      build-essential \
      ca-certificates \
      curl \
      git \
      python3 \
      python3-venv \
      wget \
    ; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*; \
    curl -fsSL https://mise.run | sh; \
    mise --version

COPY docker/sandbox-tools.toml /etc/mise/config.toml

RUN set -eux; \
    mise install; \
    mise use -g go@1.23.6 java@temurin-26.0.0 maven@3.9.9 gradle@8.12.1; \
    go version; \
    java -version; \
    mvn -version; \
    gradle -version

WORKDIR /workspace

CMD ["sleep", "infinity"]
