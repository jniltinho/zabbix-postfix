#!/usr/bin/env bash

# For Debian/Ubuntu
[ -f /var/log/mail.log ] && MAILLOG=/var/log/mail.log
# For RHEL/CentOS
[ -f /var/log/maillog ] && MAILLOG=/var/log/maillog

# Go binary paths — override via env for testing
PYGTAIL=${ZABBIX_POSTFIX_PYGTAIL:-/opt/zabbix_postfix/pygtail}
PFLOGSUMM=${ZABBIX_POSTFIX_PFLOGSUMM:-/opt/zabbix_postfix/pflogsumm}

PFOFFSETFILE=/tmp/zabbix-postfix-passive-offset.dat
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

# check for Go binaries required to run the script
if [ ! -x "${PFLOGSUMM}" ]; then
        echo "ERROR: ${PFLOGSUMM} not found — run 'make install' from the repo root"
        exit 1
fi

if [ ! -x "${PYGTAIL}" ]; then
        echo "ERROR: ${PYGTAIL} not found — run 'make install' from the repo root"
        exit 1
fi

if [ ! -r "${MAILLOG}" ]; then
        echo "ERROR: ${MAILLOG} not readable"
        exit 1
fi

# check whether stats file exists and is writable
if [ ! -w "${PFSTATSFILE}" ]; then
        touch "${PFSTATSFILE}" > /dev/null 2>&1
        if [ ! $? -eq 0 ]; then
                result_text="ERROR: could not create stats file ${PFSTATSFILE}"
                result_code="1"
                write_result "${result_code}" "${result_text}"
        fi
fi

# read specific value from stats file and print it
readvalue () {
        key=$(echo "${PFVALS[@]}" | grep -wo "$1")
        if [ -n "${key}" ]; then
                value=$(grep -e "^${key};" "${PFSTATSFILE}" | cut -d ";" -f2)
                echo "${value}"
        else
                rm -f "${TEMPFILE}"
                result_text="ERROR: could not get value \"$1\" from ${PFSTATSFILE}"
                result_code="1"
                write_result "${result_code}" "${result_text}"
        fi
}

# parse key=value output from pflogsumm-go and accumulate into stats file
updatevalue() {
        key=$1

        # pflogsumm-go outputs integers directly — no k/m/g conversion needed
        value=$(grep -m1 "^${key}=" "${TEMPFILE}" | cut -d= -f2)

        if [ -z "${value}" ]; then
                return
        fi

        old_value=$(grep -e "^${key};" "${PFSTATSFILE}" | cut -d ";" -f2)
        if [ -n "${old_value}" ]; then
                sed -i -e "s/^${key};${old_value}/${key};$((old_value + value))/" "${PFSTATSFILE}"
        else
                echo "${key};${value}" >> "${PFSTATSFILE}"
        fi
}

# is there a request for a specific value or do we update all values?
if [ -n "$1" ]; then
        readvalue "$1"
else
        # read new lines from mail log and parse with pflogsumm (key=value output)
        "${PYGTAIL}" -o"${PFOFFSETFILE}" "${MAILLOG}" | \
                "${PFLOGSUMM}" --zabbix -u 0 --no_bounce_detail --no_deferral_detail \
                --no_reject_detail --no_smtpd_warnings --no_no_msg_size \
                > "${TEMPFILE}" 2>/dev/null

        if [ ! $? -eq 0 ]; then
                result_text="ERROR: pipeline failed: ${PYGTAIL} | ${PFLOGSUMM}"
                result_code="1"
                write_result "${result_code}" "${result_text}"
        fi

        for i in "${PFVALS[@]}"; do
                updatevalue "$i"
        done

        result_text="OK: statistics updated"
        result_code="0"
        write_result "${result_code}" "${result_text}"
fi
rm -f "${TEMPFILE}"
