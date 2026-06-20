# Build and integration-test check_mailq against the Perl nagios-plugins reference.
#
#   Build + run integration tests:
#     docker build -t check-mailq-test .
#     docker run --rm check-mailq-test
#
#   Build only (stop after Go unit tests):
#     docker build --target build .

# ── Stage 1: compile Go binary + unit tests ───────────────────────────────────
FROM golang:1.26.4-bookworm AS build

WORKDIR /src

# Download dependencies first (layer cache-friendly)
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Unit tests
RUN go test ./...

# Statically linked binary
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /check_mailq .

# ── Stage 2: integration tests on ubuntu:24.04 ────────────────────────────────
FROM ubuntu:24.04

# monitoring-plugins provides:
#   /usr/lib/nagios/plugins/check_mailq   Perl reference implementation
#   /usr/lib/nagios/plugins/utils.pm      Nagios utilities module (PATH_TO_MAILQ etc.)
# postfix provides:
#   /usr/bin/mailq                         real mailq binary (used by utils.pm)
RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        monitoring-plugins \
        postfix \
        sudo \
        perl \
    && rm -rf /var/lib/apt/lists/*

# Go binary under test — installed to /usr/local/bin (does not replace the Perl plugin)
COPY --from=build /check_mailq /usr/local/bin/check_mailq

# Integration test script
COPY integration-test.sh /integration-test.sh
RUN chmod +x /integration-test.sh

# Show versions for troubleshooting
RUN echo "=== Perl check_mailq ===" && \
    /usr/lib/nagios/plugins/check_mailq --version 2>&1 || true && \
    echo "=== Go check_mailq ===" && \
    /usr/local/bin/check_mailq --version && \
    echo "=== utils.pm PATH_TO_MAILQ ===" && \
    grep "PATH_TO_MAILQ" /usr/lib/nagios/plugins/utils.pm | head -3

CMD ["/integration-test.sh"]
