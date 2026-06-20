#!/usr/bin/env bash
set -euo pipefail

COL_ESCAPE="\033[0m"
GREEN="\033[0;32m"
RED="\033[0;31m"
ERR_HIGHLIGHT="\033[1;37;101m"
OK_HIGHLIGHT="\033[1;30;102m"
INFO_HIGHLIGHT="\033[1;30;104m"

clear
echo -e "${INFO_HIGHLIGHT}"
echo "  zabbix-postfix — passive agent installer  "
echo "  https://github.com/jniltinho/zabbix-postfix  "
echo -e "${COL_ESCAPE}"

while true; do
    read -rp "Install Postfix passive agent check for Zabbix? (Y/n) " yn
    case $yn in
        [Yy]*|"" ) echo "Starting install..."; break;;
        [Nn]* ) echo -e "${ERR_HIGHLIGHT}Aborted.${COL_ESCAPE}"; exit 0;;
        * ) echo "Please answer Y or n.";;
    esac
done

if [[ $(id -u) -ne 0 ]]; then
    echo -e "\n${ERR_HIGHLIGHT}ERROR: Run as root (sudo).${COL_ESCAPE}\n"
    exit 1
fi

# --- Check Go binaries ---
echo -e "\nChecking Go binary dependencies..."
GO_BINS=( "/opt/zabbix_postfix/pygtail" "/opt/zabbix_postfix/pflogsumm" "/opt/zabbix_postfix/check_mailq" )
missing=0
for bin in "${GO_BINS[@]}"; do
    printf "  %-40s" "${bin}"
    if [ -x "${bin}" ]; then
        echo -e "[${GREEN}OK${COL_ESCAPE}]"
    else
        echo -e "[${RED}MISSING${COL_ESCAPE}]"
        missing=$((missing + 1))
    fi
done

if [ $missing -gt 0 ]; then
    echo -e "\n${ERR_HIGHLIGHT}${missing} Go binary/binaries missing.${COL_ESCAPE}"
    echo "Build and install from the repo root:"
    echo "  cd /path/to/zabbix-postfix && make build && sudo make install"
    exit 1
fi
echo -e "${OK_HIGHLIGHT}All Go binaries found.${COL_ESCAPE}\n"

# --- Install passive script ---
SCRIPT_SRC="$(dirname "$0")/zabbix_postfix_passive.sh"
[ -f "./zabbix_postfix_passive.sh" ] && SCRIPT_SRC="./zabbix_postfix_passive.sh"

printf "  Installing /opt/zabbix_postfix/zabbix_postfix_passive.sh ... "
if install -m 0755 "${SCRIPT_SRC}" /opt/zabbix_postfix/zabbix_postfix_passive.sh; then
    echo -e "[${GREEN}OK${COL_ESCAPE}]"
else
    echo -e "[${RED}FAILED${COL_ESCAPE}]"
    exit 1
fi

# --- Detect Zabbix agent config directory ---
zbx_conf_dir=""
zbx_service=""

if [ -d "/etc/zabbix/zabbix_agent2.d" ]; then
    zbx_conf_dir="/etc/zabbix/zabbix_agent2.d"
    zbx_service="zabbix-agent2"
elif [ -d "/etc/zabbix/zabbix_agentd.conf.d" ]; then
    zbx_conf_dir="/etc/zabbix/zabbix_agentd.conf.d"
    zbx_service="zabbix-agent"
elif [ -d "/etc/zabbix/zabbix_agentd.d" ]; then
    zbx_conf_dir="/etc/zabbix/zabbix_agentd.d"
    zbx_service="zabbix-agent"
else
    echo -e "${ERR_HIGHLIGHT}Could not find Zabbix agent config directory.${COL_ESCAPE}"
    echo "Copy zabbix_postfix_passive.conf manually to the agent's conf.d directory."
    zbx_conf_dir=""
fi

CONF_SRC="$(dirname "$0")/zabbix_postfix_passive.conf"
[ -f "./zabbix_postfix_passive.conf" ] && CONF_SRC="./zabbix_postfix_passive.conf"

if [ -n "${zbx_conf_dir}" ]; then
    printf "  Installing ${zbx_conf_dir}/zabbix_postfix_passive.conf ... "
    if install -m 0644 "${CONF_SRC}" "${zbx_conf_dir}/zabbix_postfix_passive.conf"; then
        echo -e "[${GREEN}OK${COL_ESCAPE}]"
    else
        echo -e "[${RED}FAILED${COL_ESCAPE}]"
        exit 1
    fi
fi

# --- Sudoers ---
SUDOERS_LINE="zabbix ALL=(ALL) NOPASSWD: /opt/zabbix_postfix/zabbix_postfix_passive.sh"
printf "  Configuring sudoers ... "
if grep -qF "${SUDOERS_LINE}" /etc/sudoers 2>/dev/null; then
    echo -e "[${GREEN}ALREADY SET${COL_ESCAPE}]"
else
    if echo "${SUDOERS_LINE}" | EDITOR='tee -a' visudo > /dev/null 2>&1; then
        echo -e "[${GREEN}OK${COL_ESCAPE}]"
    else
        echo -e "[${RED}FAILED${COL_ESCAPE}]"
        echo "Add manually to /etc/sudoers:"
        echo "  ${SUDOERS_LINE}"
        exit 1
    fi
fi

# --- Restart Zabbix agent ---
printf "  Restarting ${zbx_service:-zabbix-agent} ... "
if systemctl restart "${zbx_service:-zabbix-agent}" 2>/dev/null; then
    echo -e "[${GREEN}OK${COL_ESCAPE}]"
else
    echo -e "[${RED}FAILED${COL_ESCAPE}]"
    echo "Restart manually: systemctl restart ${zbx_service:-zabbix-agent}"
fi

echo -e "\n${OK_HIGHLIGHT}Installation complete.${COL_ESCAPE}"
echo ""
echo "Next steps:"
echo "  1. Import template_postfix_passive.xml into the Zabbix server"
echo "     Configuration > Templates > Import"
echo "  2. Attach the template to the host in Zabbix"
echo "     Configuration > Hosts > [host] > Templates > Link new templates"
echo "  3. Verify with:"
echo "     zabbix_get -s 127.0.0.1 -k 'postfix.update_data'"
echo ""
exit 0
