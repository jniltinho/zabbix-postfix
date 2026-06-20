#!/bin/bash
# Resets the pygtail offset to the end of mail.log and clears the stats file.
# Run via Zabbix: Monitoring -> Hosts -> (host) -> Scripts -> Reset Postfix offset
LOG=/var/log/mail.log
OFFSET_FILE=/tmp/zabbix-postfix-passive-offset.dat
STATS_FILE=/tmp/zabbix-postfix-passive-statsfile.dat
INODE=$(stat -c '%i' "$LOG")
SIZE=$(stat -c '%s' "$LOG")
printf '%s\n%s\n' "$INODE" "$SIZE" > "$OFFSET_FILE"
rm -f "$STATS_FILE"
echo "Reset OK: inode=$INODE offset=$SIZE"
