#!/usr/bin/env bash
# install.sh — post-extract installer for zabbix-postfix
#
# Usage:
#   tar -xzf zabbix-postfix.tar.gz -C /tmp
#   sudo bash /tmp/usr/share/zabbix-postfix/install.sh [--bin-dir DIR] [--script-dir DIR]
#
# Options:
#   --bin-dir DIR     Install binaries (pygtail, pflogsumm, check_mailq) to DIR
#                     Default: /opt/zabbix_postfix
#   --script-dir DIR  Install zabbix_postfix_passive.sh to DIR
#                     Default: /opt/zabbix_postfix

set -e

BIN_DIR="/opt/zabbix_postfix"
SCRIPT_DIR="/opt/zabbix_postfix"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --bin-dir)    BIN_DIR="${2:?--bin-dir requires a value}";     shift 2 ;;
        --script-dir) SCRIPT_DIR="${2:?--script-dir requires a value}"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: run as root (sudo bash $0)"
    exit 1
fi

# Derive extraction root from this script's own location.
# install.sh lives at <extract_root>/usr/share/zabbix-postfix/install.sh
SELF_DIR="$(cd "$(dirname "$0")" && pwd)"
EXTRACT_ROOT="$(cd "${SELF_DIR}/../../.." && pwd)"

SRC_BIN="${EXTRACT_ROOT}/opt/zabbix_postfix"
SRC_SBIN="${EXTRACT_ROOT}/opt/zabbix_postfix"
SRC_SHARE="${EXTRACT_ROOT}/usr/share/zabbix-postfix"
DEST_SHARE="/usr/share/zabbix-postfix"

# Install binaries
mkdir -p "${BIN_DIR}"
install -m 0755 "${SRC_BIN}/pygtail"     "${BIN_DIR}/pygtail"
install -m 0755 "${SRC_BIN}/pflogsumm"   "${BIN_DIR}/pflogsumm"
install -m 0755 "${SRC_BIN}/check_mailq" "${BIN_DIR}/check_mailq"
echo "==> binaries installed to ${BIN_DIR}"

# Install script
mkdir -p "${SCRIPT_DIR}"
install -m 0755 "${SRC_SBIN}/zabbix_postfix_passive.sh" "${SCRIPT_DIR}/zabbix_postfix_passive.sh"
install -m 0755 "${SRC_SBIN}/zabbix-reset-offset.sh"   "${SCRIPT_DIR}/zabbix-reset-offset.sh"
echo "==> scripts installed to ${SCRIPT_DIR}"

# Install shared files (template + generated conf)
mkdir -p "${DEST_SHARE}"
install -m 0644 "${SRC_SHARE}/template_postfix_passive.xml" "${DEST_SHARE}/template_postfix_passive.xml"

# Generate conf with actual paths
cat > "${DEST_SHARE}/zabbix_postfix_passive.conf" <<EOF
UserParameter=postfix.pfmailq,${BIN_DIR}/check_mailq --zabbix
UserParameter=postfix[*],sudo ${SCRIPT_DIR}/zabbix_postfix_passive.sh \$1
UserParameter=postfix.update_data,sudo ${SCRIPT_DIR}/zabbix_postfix_passive.sh
EOF

# Detect Zabbix agent conf dir (agent2 → agentd.conf.d → agentd.d)
ZBX_CONF_DIR=""
ZBX_SERVICE=""
for pair in \
    "/etc/zabbix/zabbix_agent2.d:zabbix-agent2" \
    "/etc/zabbix/zabbix_agentd.conf.d:zabbix-agent" \
    "/etc/zabbix/zabbix_agentd.d:zabbix-agent"; do
    dir="${pair%%:*}"
    svc="${pair##*:}"
    if [ -d "$dir" ]; then
        ZBX_CONF_DIR="$dir"
        ZBX_SERVICE="$svc"
        break
    fi
done

if [ -n "$ZBX_CONF_DIR" ]; then
    install -m 0644 "${DEST_SHARE}/zabbix_postfix_passive.conf" "${ZBX_CONF_DIR}/zabbix_postfix_passive.conf"
    echo "==> conf installed to ${ZBX_CONF_DIR}/zabbix_postfix_passive.conf"
else
    echo "WARNING: no Zabbix agent conf dir found — copy ${DEST_SHARE}/zabbix_postfix_passive.conf manually"
fi

# Add sudoers entry (idempotent)
SUDOERS_LINE="zabbix ALL=(ALL) NOPASSWD: ${SCRIPT_DIR}/zabbix_postfix_passive.sh"
SUDOERS_RESET="zabbix ALL=(ALL) NOPASSWD: ${SCRIPT_DIR}/zabbix-reset-offset.sh"
if ! grep -qF "$SUDOERS_LINE" /etc/sudoers 2>/dev/null; then
    echo "$SUDOERS_LINE" >> /etc/sudoers
    echo "==> sudoers entry added"
fi
if ! grep -qF "$SUDOERS_RESET" /etc/sudoers 2>/dev/null; then
    echo "$SUDOERS_RESET" >> /etc/sudoers
    echo "==> sudoers reset entry added"
fi

# Restart agent
if [ -n "$ZBX_SERVICE" ] && systemctl is-active --quiet "$ZBX_SERVICE" 2>/dev/null; then
    systemctl restart "$ZBX_SERVICE" || true
    echo "==> ${ZBX_SERVICE} restarted"
fi

echo ""
echo "==> zabbix-postfix installed successfully"
echo "    Import ${DEST_SHARE}/template_postfix_passive.xml in the Zabbix UI"
