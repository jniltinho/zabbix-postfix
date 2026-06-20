#!/usr/bin/env bash
# configure_paths.sh — Reconfigure zabbix-postfix binary and script paths.
#
# Usage:
#   sudo ./configure_paths.sh --bin-dir DIR [--script-dir DIR]
#
# Options:
#   --bin-dir DIR     Directory where pygtail, pflogsumm and check_mailq live
#                     Default: /usr/local/bin
#   --script-dir DIR  Directory where zabbix_postfix_passive.sh is installed
#                     Default: /usr/local/sbin
#   -h, --help        Show this help

set -euo pipefail

COL_RESET="\033[0m"
GREEN="\033[0;32m"
RED="\033[0;31m"
YELLOW="\033[0;33m"
BOLD="\033[1m"

ok()   { echo -e "  [${GREEN}OK${COL_RESET}]  $*"; }
fail() { echo -e "  [${RED}FAIL${COL_RESET}] $*"; exit 1; }
info() { echo -e "  [${YELLOW}..${COL_RESET}]  $*"; }

# ---------- defaults ----------
BIN_DIR="/usr/local/bin"
SCRIPT_DIR="/usr/local/sbin"

# ---------- parse args ----------
while [[ $# -gt 0 ]]; do
    case "$1" in
        --bin-dir)    BIN_DIR="${2:?--bin-dir requires a value}"; shift 2 ;;
        --script-dir) SCRIPT_DIR="${2:?--script-dir requires a value}"; shift 2 ;;
        -h|--help)
            sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

SCRIPT_FILE="${SCRIPT_DIR}/zabbix_postfix_passive.sh"

echo -e "\n${BOLD}zabbix-postfix — configure paths${COL_RESET}"
echo    "  Binary dir : ${BIN_DIR}"
echo -e "  Script dir : ${SCRIPT_DIR}\n"

# ---------- root check ----------
if [[ $(id -u) -ne 0 ]]; then
    fail "Run as root (sudo $0 ...)"
fi

# ---------- check binaries ----------
info "Checking binaries in ${BIN_DIR}..."
for bin in pygtail pflogsumm check_mailq; do
    if [[ ! -x "${BIN_DIR}/${bin}" ]]; then
        fail "${BIN_DIR}/${bin} not found or not executable"
    fi
done
ok "All three binaries found in ${BIN_DIR}"

# ---------- update zabbix_postfix_passive.sh ----------
if [[ ! -f "${SCRIPT_FILE}" ]]; then
    fail "${SCRIPT_FILE} not found — run the installer first"
fi

info "Updating ${SCRIPT_FILE}..."
sed -i \
    -e "s|PYGTAIL=\${ZABBIX_POSTFIX_PYGTAIL:-[^}]*}|PYGTAIL=\${ZABBIX_POSTFIX_PYGTAIL:-${BIN_DIR}/pygtail}|" \
    -e "s|PFLOGSUMM=\${ZABBIX_POSTFIX_PFLOGSUMM:-[^}]*}|PFLOGSUMM=\${ZABBIX_POSTFIX_PFLOGSUMM:-${BIN_DIR}/pflogsumm}|" \
    "${SCRIPT_FILE}"
ok "Updated PYGTAIL and PFLOGSUMM paths"

# ---------- detect zabbix agent conf dir ----------
ZBX_CONF_FILE=""
ZBX_SERVICE=""

for candidate in \
    "/etc/zabbix/zabbix_agent2.d/zabbix_postfix_passive.conf" \
    "/etc/zabbix/zabbix_agentd.conf.d/zabbix_postfix_passive.conf" \
    "/etc/zabbix/zabbix_agentd.d/zabbix_postfix_passive.conf"
do
    if [[ -f "${candidate}" ]]; then
        ZBX_CONF_FILE="${candidate}"
        break
    fi
done

if [[ -z "${ZBX_CONF_FILE}" ]]; then
    echo -e "  [${YELLOW}SKIP${COL_RESET}] zabbix_postfix_passive.conf not found — update check_mailq path manually"
else
    info "Updating ${ZBX_CONF_FILE}..."
    sed -i "s|UserParameter=postfix\.pfmailq,.*|UserParameter=postfix.pfmailq,${BIN_DIR}/check_mailq|" \
        "${ZBX_CONF_FILE}"
    ok "Updated check_mailq path"
fi

# ---------- detect and restart agent ----------
ZBX_SERVICE=""
for svc in zabbix-agent2 zabbix-agent; do
    if systemctl is-active --quiet "${svc}" 2>/dev/null; then
        ZBX_SERVICE="${svc}"
        break
    fi
done

if [[ -z "${ZBX_SERVICE}" ]]; then
    echo -e "  [${YELLOW}SKIP${COL_RESET}] No running Zabbix agent detected — restart it manually"
else
    info "Restarting ${ZBX_SERVICE}..."
    if systemctl restart "${ZBX_SERVICE}"; then
        ok "${ZBX_SERVICE} restarted"
    else
        fail "Failed to restart ${ZBX_SERVICE}"
    fi
fi

echo -e "\n${BOLD}Done.${COL_RESET} Verify with:"
echo "  zabbix_get -s 127.0.0.1 -k 'postfix.update_data'"
echo "  zabbix_get -s 127.0.0.1 -k 'postfix.pfmailq'"
echo ""
