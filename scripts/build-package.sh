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

fpm \
    -s dir \
    -t "${FORMAT}" \
    --name "${PKG_NAME}" \
    --version "${VERSION}" \
    --architecture "${ARCH}" \
    --description "Postfix monitoring for Zabbix — pygtail, pflogsumm and check_mailq (Go binaries)" \
    --maintainer "Nilton OS <jniltinho@gmail.com>" \
    --url "https://github.com/jniltinho/zabbix-postfix" \
    --license "MIT" \
    --depends "sudo" \
    --after-install "scripts/pkg/postinst" \
    --before-remove "scripts/pkg/prerm" \
    --package "${DIST_DIR}/" \
    --force \
    "pygtail/dist/pygtail=/usr/local/bin/pygtail" \
    "pflogsumm/dist/pflogsumm=/usr/local/bin/pflogsumm" \
    "check_mailq/dist/check_mailq=/usr/local/bin/check_mailq" \
    "zabbix_postfix_passive.sh=/usr/local/sbin/zabbix_postfix_passive.sh" \
    "zabbix_postfix_passive.conf=/usr/share/zabbix-postfix/zabbix_postfix_passive.conf" \
    "template_postfix_passive.xml=/usr/share/zabbix-postfix/template_postfix_passive.xml"

echo ""
echo "Package ready in ${DIST_DIR}/"
ls -lh "${DIST_DIR}"/*.${FORMAT} 2>/dev/null || true
