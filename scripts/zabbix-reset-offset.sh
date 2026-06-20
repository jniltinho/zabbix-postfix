#!/bin/bash
# Clears the stats cache so the next poll rebuilds today's totals from scratch.
# Run via Zabbix: Monitoring -> Hosts -> (host) -> Scripts -> Reset Postfix offset
STATS_FILE=/tmp/zabbix-postfix-passive-statsfile.dat
rm -f "$STATS_FILE"
echo "Reset OK"
