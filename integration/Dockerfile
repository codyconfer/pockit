# syntax=docker/dockerfile:1
FROM golang:1.26.5-trixie@sha256:f1a132429b98724a904e9b3bdbaed399d8f923203c3e5170e6def66d0a7cc04c AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY data ./data
COPY internal ./internal
COPY examples/agent-docs ./examples/agent-docs
COPY .github/workflows ./.github/workflows
COPY integration ./integration
RUN test -z "$(gofmt -l cmd data internal integration)" \
    && go vet ./... \
    && go test ./... \
    && go test -race ./... \
    && go build -trimpath -buildvcs=false -o /out/pockit ./cmd/pockit \
    && go test -c -tags=integration -o /out/packages.test ./integration

FROM node:24-trixie@sha256:5c6e2e57d7dd6652ce6f087ba0328312059d7c77f93baa8751865dbf103f1acb AS hosts
SHELL ["/bin/bash", "-euo", "pipefail", "-c"]
# These are compatibility-test versions, not minimum supported versions.
ARG PI_VERSION=1.5.0
ARG OPENCODE_VERSION=1.19.5
ARG GOOSE_VERSION=1.57.0
ARG GOOSE_SHA256=6c3a05e30e1a425dc06a410dbe24b3f3697c9721b1ea2340434ddda6b64267ae
ARG CURSOR_VERSION=2026.11.05-86047df
ARG CURSOR_SHA256=170b77786cbf4c93064d64fff12b71a178b968730561c64afc2e8e752535442e
RUN test "$(uname -m)" = x86_64 \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git \
    && rm -rf /var/lib/apt/lists/* \
    && npm install --global "@earendil-works/pi-coding-agent@${PI_VERSION}" "opencode-ai@${OPENCODE_VERSION}" \
    && npm cache clean --force
# Do not pipe an unversioned remote installer into a shell. Verify release archives.
RUN curl --fail --location --retry 3 -o /tmp/goose.tar.gz \
      "https://github.com/aaif-goose/goose/releases/download/v${GOOSE_VERSION}/goose-x86_64-unknown-linux-musl.tar.gz" \
    && echo "${GOOSE_SHA256}  /tmp/goose.tar.gz" | sha256sum --check --strict \
    && tar -xzf /tmp/goose.tar.gz -C /usr/local/bin ./goose \
    && rm /tmp/goose.tar.gz \
    && mkdir /opt/cursor-agent \
    && curl --fail --location --retry 3 -o /tmp/cursor.tar.gz \
      "https://downloads.cursor.com/lab/${CURSOR_VERSION}/linux/x64/agent-cli-package.tar.gz" \
    && echo "${CURSOR_SHA256}  /tmp/cursor.tar.gz" | sha256sum --check --strict \
    && tar -xzf /tmp/cursor.tar.gz --strip-components=1 -C /opt/cursor-agent \
    && ln -s /opt/cursor-agent/cursor-agent /usr/local/bin/agent \
    && rm /tmp/cursor.tar.gz

FROM hosts AS integration
COPY --from=build /out/pockit /out/packages.test /usr/local/bin/
COPY integration/pi-discover.mjs /opt/pockit-tests/pi-discover.mjs
ENV PI_MODULE_PATH=/usr/local/lib/node_modules/@earendil-works/pi-coding-agent/dist/index.js \
    PI_SKIP_VERSION_CHECK=1 \
    OPENCODE_DISABLE_AUTOUPDATE=1 \
    OPENCODE_DISABLE_DEFAULT_PLUGINS=1 \
    OPENCODE_DISABLE_MODELS_FETCH=1 \
    NO_COLOR=1
RUN useradd --create-home --uid 10001 pockit && mkdir /work && chown pockit:pockit /work
USER pockit
WORKDIR /work
ENTRYPOINT ["/usr/local/bin/packages.test", "-test.v", "-test.timeout=8m"]
