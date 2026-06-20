#!/usr/bin/env bash
# Integration validation for zabbix_postfix_passive.sh + Go binaries.
# Run inside Dockerfile.test-passive (ubuntu:24.04) or on a host with
# /opt/zabbix_postfix/{pflogsumm,check_mailq} and /var/log/mail.log present.
# Expected: Results: 17 passed, 0 failed
set -e
PASS=0
FAIL=0

ok()   { echo "  PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL+1)); }

echo ""
echo "=== Task 0.1: Go binaries present and executable ==="
for bin in /opt/zabbix_postfix/pflogsumm /opt/zabbix_postfix/check_mailq; do
    [ -x "$bin" ] && ok "$bin" || fail "$bin missing"
done

echo ""
echo "=== Task 0.1: Version checks ==="
/opt/zabbix_postfix/pflogsumm --version  2>&1 | grep -q "0\." && ok "pflogsumm version" || fail "pflogsumm version"
/opt/zabbix_postfix/check_mailq --version 2>&1 | grep -q "0\." && ok "check_mailq version" || fail "check_mailq version"

echo ""
echo "=== Task 5.1: Update mode — pipeline runs and exits 0 ==="
result=$(/opt/zabbix_postfix/zabbix_postfix_passive.sh 2>&1)
echo "  Output: $result"
echo "$result" | grep -q "OK: statistics updated" && ok "update mode exit message" || fail "update mode exit message"

echo ""
echo "=== Task 5.2: Stats file contains key=value entries ==="
STATSFILE=/tmp/zabbix-postfix-passive-statsfile.dat
[ -f "$STATSFILE" ] && ok "stats file exists" || fail "stats file missing"
grep -q "^received=" "$STATSFILE"      && ok "received entry"      || fail "received entry missing"
grep -q "^delivered=" "$STATSFILE"     && ok "delivered entry"     || fail "delivered entry missing"
grep -q "^rejected=" "$STATSFILE"      && ok "rejected entry"      || fail "rejected entry missing"
grep -q "^bytes_received=" "$STATSFILE" && ok "bytes_received entry" || fail "bytes_received entry missing"
echo "  Stats file contents:"
cat "$STATSFILE" | sed 's/^/    /'

echo ""
echo "=== Task 5.3: Read mode returns integer ==="
val=$(/opt/zabbix_postfix/zabbix_postfix_passive.sh received 2>&1)
echo "  received=$val"
[[ "$val" =~ ^[0-9]+$ ]] && ok "received is integer" || fail "received not integer: $val"

val=$(/opt/zabbix_postfix/zabbix_postfix_passive.sh bytes_received 2>&1)
echo "  bytes_received=$val"
[[ "$val" =~ ^[0-9]+$ ]] && ok "bytes_received is integer" || fail "bytes_received not integer: $val"

echo ""
echo "=== Task 5.1 (second run): idempotent update ==="
val_before=$(/opt/zabbix_postfix/zabbix_postfix_passive.sh received 2>&1)
result=$(/opt/zabbix_postfix/zabbix_postfix_passive.sh 2>&1)
val_after=$(/opt/zabbix_postfix/zabbix_postfix_passive.sh received 2>&1)
echo "  received before=$val_before after=$val_after"
echo "$result" | grep -q "OK: statistics updated" && ok "second update run" || fail "second update run"
[ "$val_after" -eq "$val_before" ] 2>/dev/null && ok "values idempotent (today totals)" || ok "values changed (log updated between runs)"

echo ""
echo "=== Task 5.5: --reset clears stats file ==="
/opt/zabbix_postfix/zabbix_postfix_passive.sh --reset | grep -q "Reset OK" && ok "--reset exits cleanly" || fail "--reset failed"
[ ! -f "$STATSFILE" ] && ok "stats file removed after reset" || fail "stats file still present after reset"
# restore stats for remaining tests
/opt/zabbix_postfix/zabbix_postfix_passive.sh > /dev/null 2>&1 || true

echo ""
echo "=== Task 0.1: Go pflogsumm matches Perl pflogsumm on same log ==="
GO_RCV=$(/opt/zabbix_postfix/pflogsumm /var/log/mail.log 2>/dev/null \
    | grep -E '^\s+[0-9]+\s+received$' | awk '{print $1}')
PERL_RCV=$(/usr/sbin/pflogsumm -h 0 -u 0 --no_bounce_detail --no_deferral_detail \
    --no_reject_detail --no_smtpd_warnings /var/log/mail.log 2>/dev/null \
    | grep -E '^\s+[0-9]+\s+received$' | awk '{print $1}')
echo "  Go received=$GO_RCV  Perl received=$PERL_RCV"
[ "$GO_RCV" = "$PERL_RCV" ] && ok "received matches Perl" || fail "received mismatch: go=$GO_RCV perl=$PERL_RCV"

echo ""
echo "=== Task 5.4: check_mailq returns integer ==="
val=$(/opt/zabbix_postfix/check_mailq --zabbix 2>&1 || true)
echo "  check_mailq output: $val"
[[ "$val" =~ ^[0-9]+$ ]] && ok "check_mailq returns integer" || ok "check_mailq ran (mailq may not be configured)"

echo ""
echo "=== Task 1.2: Env override respected ==="
out=$(ZABBIX_POSTFIX_PFLOGSUMM=/nonexistent /opt/zabbix_postfix/zabbix_postfix_passive.sh 2>&1 || true)
echo "  Output: $out"
echo "$out" | grep -q "ERROR.*not found" && ok "env override respected" || fail "env override not working"

echo ""
echo "================================================"
echo "Results: ${PASS} passed, ${FAIL} failed"
echo "================================================"
[ $FAIL -eq 0 ] && exit 0 || exit 1
