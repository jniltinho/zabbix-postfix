#!/usr/bin/env bash
# build-binaries-docker.sh — Build Go 1.26.4 binaries via Docker (no local Go/UPX required).
#
# Usage (from repo root):
#   bash scripts/build-binaries-docker.sh
#
# Produces:
#   pflogsumm/dist/pflogsumm
#   check_mailq/dist/check_mailq

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

DOCKERFILE="${ROOT}/docs/Dockerfile"
EXPORT_DIR="${ROOT}/dist/docker-build"

if ! command -v docker >/dev/null 2>&1; then
    echo "ERROR: docker not found. Install Docker or build locally with: make build"
    exit 1
fi

echo "==> Building binaries with Docker (docs/Dockerfile)"
DOCKER_BUILDKIT=1 docker build -f "${DOCKERFILE}" --target export-bins -o "${EXPORT_DIR}" "${ROOT}"

for mod in pflogsumm check_mailq; do
    mkdir -p "${ROOT}/${mod}/dist"
    install -m 0755 "${EXPORT_DIR}/${mod}" "${ROOT}/${mod}/dist/${mod}"
done

rm -rf "${EXPORT_DIR}"

echo ""
echo "==> Binaries ready:"
ls -lh pflogsumm/dist/pflogsumm check_mailq/dist/check_mailq