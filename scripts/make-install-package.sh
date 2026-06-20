#!/usr/bin/env bash
# make-install-package.sh — Assemble a self-contained install folder for the mail server.
#
# Usage (from repo root):
#   bash scripts/make-install-package.sh              # use existing */dist/ binaries
#   bash scripts/make-install-package.sh --docker     # build via Docker first
#   bash scripts/make-install-package.sh --build      # build with local make first
#   bash scripts/make-install-package.sh --archive    # also create .tar.gz
#
# Output:
#   dist/zabbix-postfix-install/          — folder to copy to the mail server
#   dist/zabbix-postfix-install.tar.gz    — with --archive

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

DO_BUILD=0
DO_DOCKER=0
DO_ARCHIVE=0

usage() {
    sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --build)   DO_BUILD=1; shift ;;
        --docker)  DO_DOCKER=1; shift ;;
        --archive) DO_ARCHIVE=1; shift ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [[ ${DO_BUILD} -eq 1 && ${DO_DOCKER} -eq 1 ]]; then
    echo "ERROR: use only one of --build or --docker"
    exit 1
fi

if [[ ${DO_DOCKER} -eq 1 ]]; then
    bash "${ROOT}/scripts/build-binaries-docker.sh"
elif [[ ${DO_BUILD} -eq 1 ]]; then
    make build
fi

BINARIES=( pygtail pflogsumm check_mailq )
for bin in "${BINARIES[@]}"; do
    if [[ ! -x "${ROOT}/${bin}/dist/${bin}" ]]; then
        echo "ERROR: ${bin}/dist/${bin} not found."
        echo "Run one of:"
        echo "  make build"
        echo "  bash scripts/build-binaries-docker.sh"
        echo "  bash scripts/make-install-package.sh --docker"
        exit 1
    fi
done

PKG_DIR="${ROOT}/dist/zabbix-postfix-install"
rm -rf "${PKG_DIR}"
mkdir -p "${PKG_DIR}/bin" "${PKG_DIR}/scripts"

install -m 0755 "${ROOT}/pygtail/dist/pygtail"           "${PKG_DIR}/bin/"
install -m 0755 "${ROOT}/pflogsumm/dist/pflogsumm"       "${PKG_DIR}/bin/"
install -m 0755 "${ROOT}/check_mailq/dist/check_mailq"   "${PKG_DIR}/bin/"

install -m 0755 "${ROOT}/install_postfix_template_zabbix_passive.sh" "${PKG_DIR}/"
install -m 0755 "${ROOT}/zabbix_postfix_passive.sh"                  "${PKG_DIR}/"
install -m 0644 "${ROOT}/zabbix_postfix_passive.conf"                "${PKG_DIR}/"
install -m 0644 "${ROOT}/template_postfix_passive.xml"               "${PKG_DIR}/"
install -m 0755 "${ROOT}/scripts/configure_paths.sh"                   "${PKG_DIR}/scripts/"

cat > "${PKG_DIR}/INSTALL.txt" <<'EOF'
zabbix-postfix — install package
================================

Contents
--------
  bin/                                    Go binaries (pygtail, pflogsumm, check_mailq)
  install_postfix_template_zabbix_passive.sh   Agent installer
  zabbix_postfix_passive.sh               Passive check script
  zabbix_postfix_passive.conf             Zabbix agent UserParameters
  template_postfix_passive.xml            Zabbix 6.0 template (import on server)
  scripts/configure_paths.sh              Optional: custom binary paths

Mail server (Zabbix agent host)
-------------------------------
  1. Copy this folder to the mail server, e.g. /tmp/zabbix-postfix-install
  2. Install binaries:
       sudo install -m 0755 bin/pygtail bin/pflogsumm bin/check_mailq /opt/zabbix_postfix/
  3. Run the agent installer (from this directory):
       cd /tmp/zabbix-postfix-install
       sudo bash install_postfix_template_zabbix_passive.sh
  4. Verify:
       /opt/zabbix_postfix/pygtail --version
       zabbix_get -s 127.0.0.1 -k 'postfix.update_data'

Zabbix server
-------------
  1. Configuration → Templates → Import
  2. Upload template_postfix_passive.xml
  3. Link template "Template App Postfix by Zabbix agent" to the mail host

Full guide: https://github.com/jniltinho/zabbix-postfix/blob/main/docs/HOWTO.md
EOF

echo "==> Install package ready: ${PKG_DIR}/"
echo ""
find "${PKG_DIR}" -type f | sort | sed "s|${PKG_DIR}/|  |"

if [[ ${DO_ARCHIVE} -eq 1 ]]; then
    ARCHIVE="${ROOT}/dist/zabbix-postfix-install.tar.gz"
    tar -czf "${ARCHIVE}" -C "${ROOT}/dist" zabbix-postfix-install
    echo ""
    echo "==> Archive: ${ARCHIVE}"
    ls -lh "${ARCHIVE}"
fi