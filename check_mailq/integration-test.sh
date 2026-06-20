#!/bin/bash
# Integration test: validates Go check_mailq output against the Perl
# nagios-plugins reference implementation side by side.
#
# Expects:
#   /usr/lib/nagios/plugins/check_mailq   Perl reference (monitoring-plugins)
#   /usr/lib/nagios/plugins/utils.pm      Nagios utilities module
#   /usr/local/bin/check_mailq            Go binary under test
#   /usr/bin/mailq                        real mailq binary (postfix)

set -euo pipefail

PERL=/usr/lib/nagios/plugins/check_mailq
GO=/usr/local/bin/check_mailq
PASS=0
FAIL=0

# ── colour helpers ─────────────────────────────────────────────────────────────
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
RESET='\033[0m'

ok()      { echo -e "  ${GREEN}PASS${RESET}  $1"; PASS=$((PASS+1)); }
fail()    { echo -e "  ${RED}FAIL${RESET}  $1"; FAIL=$((FAIL+1)); }
info()    { echo -e "  ${YELLOW}INFO${RESET}  $1"; }
section() { echo ""; echo "── $1 ──────────────────────────────────────────"; }

# Extract status word (OK/WARNING/CRITICAL/UNKNOWN) from first word of output
status_of() { echo "$1" | grep -oE '^(OK|WARNING|CRITICAL|UNKNOWN)' || echo "NONE"; }

# Extract message count from perfdata: unsent=N;...
count_of()  { echo "$1" | grep -oP '(?<=unsent=)\d+' || echo "-1"; }

# ── discover utils.pm PATH_TO_MAILQ at runtime ────────────────────────────────
section "Environment"

MAILQ_PATH=$(perl -e 'use lib "/usr/lib/nagios/plugins"; use utils; print $utils::PATH_TO_MAILQ' 2>/dev/null \
             || echo "/usr/bin/mailq")
GO_VER=$("$GO" --version 2>&1 || echo "unknown")
PKG_VER=$(dpkg -s monitoring-plugins 2>/dev/null | grep '^Version' | awk '{print $2}' || echo "unknown")

info "utils.pm PATH_TO_MAILQ  = $MAILQ_PATH"
info "Perl plugin              = $PERL"
info "Go binary                = $GO ($GO_VER)"
info "monitoring-plugins       = $PKG_VER"
info "postfix mailq            = $(command -v mailq 2>/dev/null || echo 'not found')"

# ── fake mailq generator ───────────────────────────────────────────────────────
# Produces valid Postfix mailq output with N queued messages.
# Both parsers must agree: Go counts ^[0-9A-Z] header lines; Perl reads
# the summary line "-- N Kbytes in N Requests."
#
# Note: use echo (not printf) for the summary line — dash's printf treats
# a format string starting with "--" as an illegal option (exits 2).
MAILQ_GEN=/usr/local/bin/mailq-gen
cat > "$MAILQ_GEN" <<'SCRIPT'
#!/bin/sh
N=${1:-0}
if [ "$N" -eq 0 ]; then
    echo "Mail queue is empty"
    exit 0
fi
echo "-Queue ID-  --Size-- ----Arrival Time---- -Sender/Recipient-------"
i=1
while [ "$i" -le "$N" ]; do
    printf 'A%09X      1024 Sat Jun 20 10:00:%02d  sender%d@example.com\n' "$i" "$i" "$i"
    printf '                                         dest%d@example.com\n\n' "$i"
    i=$((i+1))
done
echo "-- ${N} Kbytes in ${N} Requests."
exit 0
SCRIPT
chmod +x "$MAILQ_GEN"

# Back up the real mailq (from postfix) before any test overwrites it.
MAILQ_REAL_BAK=/usr/bin/mailq.real
if [[ -f "$MAILQ_PATH" && ! -f "$MAILQ_REAL_BAK" ]]; then
    cp "$MAILQ_PATH" "$MAILQ_REAL_BAK"
    info "Backed up real mailq → $MAILQ_REAL_BAK"
fi

# Install fake mailq at the path Perl reads from utils.pm ($MAILQ_PATH = /usr/bin/mailq).
# Go tests use --mailq-path pointing to the same path.
install_fake_mailq() {
    local n=$1
    mkdir -p "$(dirname "$MAILQ_PATH")"
    printf '#!/bin/sh\n%s %s\n' "$MAILQ_GEN" "$n" > "$MAILQ_PATH"
    chmod +x "$MAILQ_PATH"
}

# ── test helpers ───────────────────────────────────────────────────────────────
# Capture command output + exit code safely under set -e.
# Pattern: ec=0; out=$(run_perl ...) || ec=$?
# The "|| ec=$?" catches non-zero exit without aborting (set -e sees the ||).

run_perl() { "$PERL" -M postfix "$@" 2>/dev/null; }
run_go()   { "$GO"  --mailq-path "$MAILQ_PATH" "$@" 2>/dev/null; }

# ── Test 1: empty queue — Zabbix mode ─────────────────────────────────────────
section "Test 1 — empty queue, --zabbix mode"
install_fake_mailq 0

code=0; out=$(run_go --zabbix) || code=$?
info "Go --zabbix: output='$out' exit=$code"
[[ "$out" == "0" && "$code" -eq 0 ]] \
    && ok "Go --zabbix: prints '0', exits 0" \
    || fail "Go --zabbix: expected '0'/exit=0, got '$out'/exit=$code"

# ── Test 2: empty queue — Nagios mode (Perl + Go) ─────────────────────────────
section "Test 2 — empty queue, Nagios mode"
install_fake_mailq 0

perl_exit=0; perl_out=$(run_perl -w 5 -c 20) || perl_exit=$?
go_exit=0;   go_out=$(run_go    -w 5 -c 20) || go_exit=$?
info "Perl: $perl_out  (exit=$perl_exit)"
info "Go  : $go_out   (exit=$go_exit)"

perl_s=$(status_of "$perl_out"); go_s=$(status_of "$go_out")
perl_c=$(count_of  "$perl_out"); go_c=$(count_of  "$go_out")

[[ "$perl_s" == "OK" && "$perl_exit" -eq 0 ]] && ok "Perl: OK/0"     || fail "Perl: expected OK/0, got $perl_s/$perl_exit"
[[ "$go_s"   == "OK" && "$go_exit"   -eq 0 ]] && ok "Go:   OK/0"     || fail "Go:   expected OK/0, got $go_s/$go_exit"
[[ "$perl_c" == "0" ]]                         && ok "Perl: unsent=0" || fail "Perl: expected unsent=0, got $perl_c"
[[ "$go_c"   == "0" ]]                         && ok "Go:   unsent=0" || fail "Go:   expected unsent=0, got $go_c"

# ── Test 3: 3 messages — below warning (OK) ───────────────────────────────────
section "Test 3 — 3 messages, -w 5 -c 20 → OK"
install_fake_mailq 3

perl_exit=0; perl_out=$(run_perl -w 5 -c 20) || perl_exit=$?
go_exit=0;   go_out=$(run_go    -w 5 -c 20) || go_exit=$?
info "Perl: $perl_out  (exit=$perl_exit)"
info "Go  : $go_out   (exit=$go_exit)"

perl_s=$(status_of "$perl_out"); go_s=$(status_of "$go_out")
perl_c=$(count_of  "$perl_out"); go_c=$(count_of  "$go_out")

[[ "$perl_s" == "OK" && "$perl_exit" -eq 0 ]] && ok "Perl: OK/0"            || fail "Perl: expected OK/0, got $perl_s/$perl_exit"
[[ "$go_s"   == "OK" && "$go_exit"   -eq 0 ]] && ok "Go:   OK/0"            || fail "Go:   expected OK/0, got $go_s/$go_exit"
[[ "$perl_c" == "$go_c" ]]                     && ok "Count match: unsent=$go_c" || fail "Count mismatch: Perl=$perl_c Go=$go_c"

# ── Test 4: 3 messages — Zabbix mode ─────────────────────────────────────────
section "Test 4 — 3 messages, --zabbix mode"
install_fake_mailq 3

code=0; out=$(run_go --zabbix) || code=$?
info "Go --zabbix: output='$out' exit=$code"
[[ "$out" == "3" && "$code" -eq 0 ]] \
    && ok "Go --zabbix: prints '3', exits 0" \
    || fail "Go --zabbix: expected '3'/exit=0, got '$out'/exit=$code"

# ── Test 5: 10 messages — WARNING ─────────────────────────────────────────────
section "Test 5 — 10 messages, -w 5 -c 20 → WARNING"
install_fake_mailq 10

perl_exit=0; perl_out=$(run_perl -w 5 -c 20) || perl_exit=$?
go_exit=0;   go_out=$(run_go    -w 5 -c 20) || go_exit=$?
info "Perl: $perl_out  (exit=$perl_exit)"
info "Go  : $go_out   (exit=$go_exit)"

perl_s=$(status_of "$perl_out"); go_s=$(status_of "$go_out")
perl_c=$(count_of  "$perl_out"); go_c=$(count_of  "$go_out")

[[ "$perl_s" == "WARNING" && "$perl_exit" -eq 1 ]] && ok "Perl: WARNING/1"       || fail "Perl: expected WARNING/1, got $perl_s/$perl_exit"
[[ "$go_s"   == "WARNING" && "$go_exit"   -eq 1 ]] && ok "Go:   WARNING/1"       || fail "Go:   expected WARNING/1, got $go_s/$go_exit"
[[ "$perl_c" == "$go_c" ]]                          && ok "Count match: unsent=$go_c" || fail "Count mismatch: Perl=$perl_c Go=$go_c"

# ── Test 6: 10 messages — Zabbix mode ────────────────────────────────────────
section "Test 6 — 10 messages, --zabbix mode"
install_fake_mailq 10

code=0; out=$(run_go --zabbix) || code=$?
info "Go --zabbix: output='$out' exit=$code"
[[ "$out" == "10" && "$code" -eq 0 ]] \
    && ok "Go --zabbix: prints '10', exits 0" \
    || fail "Go --zabbix: expected '10'/exit=0, got '$out'/exit=$code"

# ── Test 7: 25 messages — CRITICAL ────────────────────────────────────────────
section "Test 7 — 25 messages, -w 5 -c 20 → CRITICAL"
install_fake_mailq 25

perl_exit=0; perl_out=$(run_perl -w 5 -c 20) || perl_exit=$?
go_exit=0;   go_out=$(run_go    -w 5 -c 20) || go_exit=$?
info "Perl: $perl_out  (exit=$perl_exit)"
info "Go  : $go_out   (exit=$go_exit)"

perl_s=$(status_of "$perl_out"); go_s=$(status_of "$go_out")
perl_c=$(count_of  "$perl_out"); go_c=$(count_of  "$go_out")

[[ "$perl_s" == "CRITICAL" && "$perl_exit" -eq 2 ]] && ok "Perl: CRITICAL/2"      || fail "Perl: expected CRITICAL/2, got $perl_s/$perl_exit"
[[ "$go_s"   == "CRITICAL" && "$go_exit"   -eq 2 ]] && ok "Go:   CRITICAL/2"      || fail "Go:   expected CRITICAL/2, got $go_s/$go_exit"
[[ "$perl_c" == "$go_c" ]]                           && ok "Count match: unsent=$go_c" || fail "Count mismatch: Perl=$perl_c Go=$go_c"

# ── Test 8: missing -w/-c → UNKNOWN ──────────────────────────────────────────
section "Test 8 — missing required flags → UNKNOWN exit=3"
install_fake_mailq 3

go_exit=0; go_out=$(run_go) || go_exit=$?
info "Go (no flags): '$go_out'  exit=$go_exit"
go_s=$(status_of "$go_out")
[[ "$go_s" == "UNKNOWN" && "$go_exit" -eq 3 ]] \
    && ok "Go: UNKNOWN/3 when -w/-c missing" \
    || fail "Go: expected UNKNOWN/3, got $go_s/$go_exit"

# ── Test 9: -w >= -c → UNKNOWN ───────────────────────────────────────────────
section "Test 9 — -w >= -c → UNKNOWN exit=3"
install_fake_mailq 3

go_exit=0; go_out=$(run_go -w 20 -c 5) || go_exit=$?
info "Go (-w 20 -c 5): '$go_out'  exit=$go_exit"
go_s=$(status_of "$go_out")
[[ "$go_s" == "UNKNOWN" && "$go_exit" -eq 3 ]] \
    && ok "Go: UNKNOWN/3 when w>=c" \
    || fail "Go: expected UNKNOWN/3, got $go_s/$go_exit"

# ── Test 10: -M sendmail → UNKNOWN ────────────────────────────────────────────
section "Test 10 — unsupported -M sendmail → UNKNOWN exit=3"
install_fake_mailq 3

go_exit=0; go_out=$(run_go -M sendmail -w 5 -c 20) || go_exit=$?
info "Go (-M sendmail): '$go_out'  exit=$go_exit"
go_s=$(status_of "$go_out")
[[ "$go_s" == "UNKNOWN" && "$go_exit" -eq 3 ]] \
    && ok "Go: UNKNOWN/3 for -M sendmail" \
    || fail "Go: expected UNKNOWN/3, got $go_s/$go_exit"

# ── Test 11: -M postfix → same as no -M ──────────────────────────────────────
section "Test 11 — -M postfix accepted, same result as no -M"
install_fake_mailq 10

go_exit1=0; go_out1=$(run_go           -w 5 -c 20) || go_exit1=$?
go_exit2=0; go_out2=$(run_go -M postfix -w 5 -c 20) || go_exit2=$?
info "Go (no -M)      : $go_out1  exit=$go_exit1"
info "Go (-M postfix) : $go_out2  exit=$go_exit2"
[[ "$go_out1" == "$go_out2" && "$go_exit1" -eq "$go_exit2" ]] \
    && ok "Go: -M postfix identical to no -M" \
    || fail "Go: -M postfix differs from no -M"

# ── Test 12: -s / --sudo flag ─────────────────────────────────────────────────
section "Test 12 — -s (--sudo) flag accepted (running as root in Docker)"
install_fake_mailq 3

# Inside Docker we run as root so sudo succeeds trivially; just verify flag parses
code=0; out=$(run_go --zabbix -s) || code=$?
info "Go --zabbix -s: output='$out' exit=$code"
[[ "$out" == "3" && "$code" -eq 0 ]] \
    && ok "Go: -s flag accepted, correct count" \
    || fail "Go: -s flag, expected '3'/exit=0, got '$out'/exit=$code"

# ── Test 13: -v / --verbose flag ──────────────────────────────────────────────
section "Test 13 — -v (--verbose) debug output"
install_fake_mailq 5

go_exit=0; go_out=$(run_go -v -w 2 -c 20) || go_exit=$?
info "Go -v output:"
echo "$go_out" | sed 's/^/    /'
echo "$go_out" | grep -q "Running:"  && ok "Go -v: 'Running:' line present"  || fail "Go -v: missing 'Running:' line"
echo "$go_out" | grep -q "msg_q"     && ok "Go -v: 'msg_q' line present"     || fail "Go -v: missing 'msg_q' line"

# ── Test 14: -t / --timeout (seconds) ────────────────────────────────────────
section "Test 14 — -t (timeout in seconds, Perl-compatible)"
install_fake_mailq 3

code=0; out=$(run_go --zabbix -t 30) || code=$?
info "Go --zabbix -t 30: output='$out' exit=$code"
[[ "$out" == "3" && "$code" -eq 0 ]] \
    && ok "Go: -t flag accepted, correct count" \
    || fail "Go: -t flag, expected '3'/exit=0, got '$out'/exit=$code"

# ── Test 15: -W / -C compat flags ────────────────────────────────────────────
section "Test 15 — -W/-C domain compat flags (accepted, ignored for postfix)"
install_fake_mailq 10

go_exit=0; go_out=$(run_go -w 5 -c 20 -W 3 -C 8) || go_exit=$?
info "Go (-W 3 -C 8): '$go_out'  exit=$go_exit"
go_s=$(status_of "$go_out")
[[ "$go_s" == "WARNING" && "$go_exit" -eq 1 ]] \
    && ok "Go: -W/-C accepted without error (WARNING/1 as expected)" \
    || fail "Go: -W/-C caused unexpected result: $go_s/$go_exit"

# ── Test 16: real postfix mailq (empty queue) ─────────────────────────────────
section "Test 16 — real postfix mailq binary (empty queue after install)"

if [[ -f "$MAILQ_REAL_BAK" ]]; then
    # Restore the original postfix mailq binary
    cp "$MAILQ_REAL_BAK" "$MAILQ_PATH"
    chmod +x "$MAILQ_PATH"
    info "Restored real postfix mailq from $MAILQ_REAL_BAK"

    go_exit=0;   go_out=$(run_go --zabbix) || go_exit=$?
    perl_exit=0; perl_out=$(run_perl -w 100 -c 200) || perl_exit=$?
    info "Go  --zabbix: '$go_out' exit=$go_exit"
    info "Perl -w100 -c200: '$perl_out' exit=$perl_exit"

    if [[ "$go_exit" -eq 0 && "$go_out" =~ ^[0-9]+$ ]]; then
        # Postfix is running — verify both produce a Nagios status
        ok "Go: real mailq returns integer ($go_out), exits 0"
        perl_s=$(status_of "$perl_out")
        [[ "$perl_s" =~ ^(OK|WARNING|CRITICAL)$ ]] \
            && ok "Perl: real mailq returns Nagios status ($perl_s)" \
            || fail "Perl: unexpected status from real mailq: '$perl_out'"
    else
        # Postfix not running (common in Docker) — both tools must fail gracefully
        # Go returns UNKNOWN/3; Perl returns CRITICAL with error code from mailq
        info "Postfix not running in this container (mailq exit=$go_exit) — verifying graceful failure"
        go_s=$(status_of "$go_out")
        perl_s=$(status_of "$perl_out")
        [[ "$go_exit" -eq 3 ]] \
            && ok "Go: mailq unavailable → UNKNOWN/3 (graceful)" \
            || fail "Go: mailq unavailable but got exit=$go_exit (expected 3)"
        [[ "$perl_s" =~ ^(OK|WARNING|CRITICAL|UNKNOWN)$ || "$perl_out" =~ "Error code" ]] \
            && ok "Perl: mailq unavailable → handled gracefully ($perl_out)" \
            || fail "Perl: unexpected output when mailq unavailable: '$perl_out'"
    fi
else
    info "Real mailq backup not found — skipping real mailq test"
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════════════════════"
TOTAL=$((PASS+FAIL))
if [ "$FAIL" -eq 0 ]; then
    echo -e "${GREEN}All $TOTAL tests passed.${RESET}"
    exit 0
else
    echo -e "${RED}$FAIL/$TOTAL tests FAILED.${RESET}"
    exit 1
fi
