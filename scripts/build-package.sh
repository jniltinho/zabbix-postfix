#!/usr/bin/env bash
# build-package.sh — build a .deb or .rpm package for zabbix-postfix
#
# Usage:
#   bash scripts/build-package.sh <deb|rpm> <version>
#
# Requires: fpm (gem install fpm)
#   For rpm on Debian/Ubuntu: sudo apt-get install -y rpm

set -euo pipefail

FORMAT="${1:?Usage: build-package.sh <deb|rpm> <version>}"
VERSION="${2:?Usage: build-package.sh <deb|rpm> <version>}"

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PKG_NAME="zabbix-postfix"
ARCH="amd64"
DIST_DIR="${REPO_ROOT}/dist"

cd "${REPO_ROOT}"

if ! command -v fpm &>/dev/null; then
    echo "ERROR: fpm not found — install with: gem install fpm"
    exit 1
fi

mkdir -p "${DIST_DIR}"

FPM_ARGS=(
    -s dir
    -t "${FORMAT}"
    --name "${PKG_NAME}"
    --version "${VERSION}"
    --architecture "${ARCH}"
    --description "Postfix monitoring for Zabbix — pygtail, pflogsumm and check_mailq (Go binaries)"
    --maintainer "Nilton OS <jniltinho@gmail.com>"
    --url "https://github.com/jniltinho/zabbix-postfix"
    --license "MIT"
    --depends "sudo"
    --after-install "scripts/pkg/postinst"
    --before-remove "scripts/pkg/prerm"
    --package "${DIST_DIR}/"
    --force
    "pygtail/dist/pygtail=/opt/zabbix_postfix/pygtail"
    "pflogsumm/dist/pflogsumm=/opt/zabbix_postfix/pflogsumm"
    "check_mailq/dist/check_mailq=/opt/zabbix_postfix/check_mailq"
    "zabbix_postfix_passive.sh=/opt/zabbix_postfix/zabbix_postfix_passive.sh"
    "zabbix_postfix_passive.conf=/usr/share/zabbix-postfix/zabbix_postfix_passive.conf"
    "template_postfix_passive.xml=/usr/share/zabbix-postfix/template_postfix_passive.xml"
)

if [[ "${FORMAT}" == "tar" ]]; then
    FPM_ARGS+=("scripts/pkg/install.sh=/usr/share/zabbix-postfix/install.sh")
fi

RUBYOPT="-Eutf-8:utf-8" fpm "${FPM_ARGS[@]}"

if [[ "${FORMAT}" == "tar" ]]; then
    gzip -f "${DIST_DIR}"/*.tar
fi

echo ""
echo "Package ready in ${DIST_DIR}/"
EXT="${FORMAT}"; [[ "${FORMAT}" == "tar" ]] && EXT="tar.gz"
ls -lh "${DIST_DIR}"/*.${EXT} 2>/dev/null || true
