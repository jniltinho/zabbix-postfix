# Build zabbix-postfix Go 1.26.4 binaries without Go or UPX on the host.
#
# Run from the repository root:
#   docker build -f docs/Dockerfile --target export -o dist/docker-build .
#
# Or use the helper script (copies binaries into */dist/):
#   bash scripts/build-binaries-docker.sh

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

# Export stage — used with BuildKit: docker build --target export -o <dir> .
FROM scratch AS export
COPY --from=builder /src/pygtail/dist/pygtail /pygtail
COPY --from=builder /src/pflogsumm/dist/pflogsumm /pflogsumm
COPY --from=builder /src/check_mailq/dist/check_mailq /check_mailq