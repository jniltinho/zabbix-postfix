#!/usr/bin/env bash
# build-packages-docker.sh — Build .deb and .rpm packages via Docker (no local Go/UPX/fpm required).
#
# Usage (from repo root):
#   bash scripts/build-packages-docker.sh [VERSION]
#
# Produces:
#   dist/zabbix-postfix_<version>_amd64.deb
#   dist/zabbix-postfix-<version>-1.x86_64.rpm

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

DOCKERFILE="${ROOT}/docs/Dockerfile"
DIST_DIR="${ROOT}/dist"

VERSION="${1:-}"

if ! command -v docker >/dev/null 2>&1; then
    echo "ERROR: docker not found. Install Docker or build locally with: make pkg"
    exit 1
fi

if [[ -z "${VERSION}" ]]; then
    VERSION="$(git describe --tags --abbrev=0 2>/dev/null || echo "0.0.0")"
fi

echo "==> Building .deb and .rpm packages with Docker (version: ${VERSION})"

mkdir -p "${DIST_DIR}"

DOCKER_BUILDKIT=1 docker build \
    -f "${DOCKERFILE}" \
    --target export-pkg \
    --build-arg "VERSION=${VERSION}" \
    -o "${DIST_DIR}" \
    "${ROOT}"

echo ""
echo "==> Packages ready:"
ls -lh "${DIST_DIR}"/*.deb "${DIST_DIR}"/*.rpm "${DIST_DIR}"/*.tar.gz 2>/dev/null || echo "(no packages found in ${DIST_DIR})"
