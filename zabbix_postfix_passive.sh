#!/usr/bin/env bash

# For Debian/Ubuntu
[ -f /var/log/mail.log ] && MAILLOG=/var/log/mail.log
# For RHEL/CentOS
[ -f /var/log/maillog ] && MAILLOG=/var/log/maillog

# Go binary path — override via env for testing
PFLOGSUMM=${ZABBIX_POSTFIX_PFLOGSUMM:-/opt/zabbix_postfix/pflogsumm}

PFSTATSFILE=/tmp/zabbix-postfix-passive-statsfile.dat
TEMPFILE=$(mktemp --suffix=-zabbix-postfix-passive)

# list of values we are interested in
PFVALS=( 'received' 'delivered' 'forwarded' 'deferred' 'bounced' 'rejected' 'held' 'discarded' 'reject_warnings' 'bytes_received' 'bytes_delivered' )

# write result of running this script
write_result () {
        echo "$2"
        rm -f "${TEMPFILE}"
        exit $1
}

# --reset: clear stats cache (next poll rebuilds from today's log)
if [ "$1" = "--reset" ]; then
        rm -f "${PFSTATSFILE}"
        echo "Reset OK"
        exit 0
fi

# check for Go binary required to run the script
if [ ! -x "${PFLOGSUMM}" ]; then
        echo "ERROR: ${PFLOGSUMM} not found — run 'make install' from the repo root"
        exit 1
fi

if [ ! -r "${MAILLOG}" ]; then
        echo "ERROR: ${MAILLOG} not readable"
        exit 1
fi

# read specific value from stats file and print it
readvalue () {
        key=$(echo "${PFVALS[@]}" | grep -wo "$1")
        if [ -n "${key}" ]; then
                value=$(grep -e "^${key}=" "${PFSTATSFILE}" | cut -d= -f2)
                echo "${value}"
        else
                rm -f "${TEMPFILE}"
                result_text="ERROR: could not get value \"$1\" from ${PFSTATSFILE}"
                result_code="1"
                write_result "${result_code}" "${result_text}"
        fi
}

# is there a request for a specific value or do we update all values?
if [ -n "$1" ]; then
        readvalue "$1"
else
        # parse today's mail log with pflogsumm and cache the result
        "${PFLOGSUMM}" --zabbix -d today -u 0 --no_bounce_detail --no_deferral_detail \
                --no_reject_detail --no_smtpd_warnings --no_no_msg_size \
                "${MAILLOG}" > "${TEMPFILE}" 2>/dev/null

        if [ ! $? -eq 0 ]; then
                result_text="ERROR: pflogsumm failed on ${MAILLOG}"
                result_code="1"
                write_result "${result_code}" "${result_text}"
        fi

        cp "${TEMPFILE}" "${PFSTATSFILE}"

        result_text="OK: statistics updated"
        result_code="0"
        write_result "${result_code}" "${result_text}"
fi
rm -f "${TEMPFILE}"
