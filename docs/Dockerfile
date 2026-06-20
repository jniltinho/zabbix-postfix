# Build zabbix-postfix Go binaries and generate .deb / .rpm packages
# without Go, UPX, or fpm on the host.
#
# Build binaries only (copies to dist/docker-build/):
#   docker build -f docs/Dockerfile --target export-bins -o dist/docker-build .
#
# Build .deb and .rpm packages (copies to dist/):
#   docker build -f docs/Dockerfile --target export-pkg -o dist/ .
#
# Or use the helper scripts:
#   bash scripts/build-binaries-docker.sh
#   bash scripts/build-package.sh deb <version>   # after local build
#   bash scripts/make-install-package.sh --docker --archive

# ── stage 1: compile binaries ────────────────────────────────────────────────
FROM golang:1.26.4-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    make \
    wget \
    xz-utils \
    && rm -rf /var/lib/apt/lists/* \
    && wget -q https://github.com/upx/upx/releases/download/v4.2.4/upx-4.2.4-amd64_linux.tar.xz \
    && tar -xf upx-4.2.4-amd64_linux.tar.xz \
    && mv upx-4.2.4-amd64_linux/upx /usr/local/bin/upx \
    && rm -rf upx-4.2.4-amd64_linux upx-4.2.4-amd64_linux.tar.xz

WORKDIR /src

# Download deps first (cache-friendly layer)
COPY Makefile ./
COPY pygtail/go.mod pygtail/go.sum pygtail/
COPY pflogsumm/go.mod pflogsumm/go.sum pflogsumm/
COPY check_mailq/go.mod check_mailq/go.sum check_mailq/

RUN cd pygtail && go mod download && \
    cd ../pflogsumm && go mod download && \
    cd ../check_mailq && go mod download

COPY pygtail/ pygtail/
COPY pflogsumm/ pflogsumm/
COPY check_mailq/ check_mailq/

RUN make clean build

# ── stage 2: build .deb and .rpm ─────────────────────────────────────────────
FROM debian:bookworm-slim AS packager

RUN apt-get update && apt-get install -y --no-install-recommends \
    ruby-dev \
    rpm \
    build-essential \
    git \
    && gem install fpm --no-document \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Copy everything needed by build-package.sh and fpm
COPY --from=builder /src/pygtail/dist/pygtail       pygtail/dist/pygtail
COPY --from=builder /src/pflogsumm/dist/pflogsumm   pflogsumm/dist/pflogsumm
COPY --from=builder /src/check_mailq/dist/check_mailq check_mailq/dist/check_mailq
COPY Makefile ./
COPY zabbix_postfix_passive.sh ./
COPY zabbix_postfix_passive.conf ./
COPY template_postfix_passive.xml ./
COPY scripts/ scripts/

# VERSION is passed at build time; falls back to git tag via Makefile if not set
ARG VERSION=0.0.0

RUN mkdir -p dist && \
    bash scripts/build-package.sh deb "${VERSION}" && \
    bash scripts/build-package.sh rpm "${VERSION}" && \
    bash scripts/build-package.sh tar "${VERSION}"

# ── export stage: binaries only ──────────────────────────────────────────────
FROM scratch AS export-bins
COPY --from=builder /src/pygtail/dist/pygtail       /pygtail
COPY --from=builder /src/pflogsumm/dist/pflogsumm   /pflogsumm
COPY --from=builder /src/check_mailq/dist/check_mailq /check_mailq

# ── export stage: .deb, .rpm and .tar.gz packages ────────────────────────────
FROM scratch AS export-pkg
COPY --from=packager /src/dist/*.deb /
COPY --from=packager /src/dist/*.rpm /
COPY --from=packager /src/dist/*.tar.gz /
