## 0. Prerequisites

- [x] 0.1 Verify `converter-to-golang` is implemented: `/usr/local/bin/pygtail`, `/usr/local/bin/pflogsumm`, `/usr/local/bin/check_mailq` exist and pass golden tests
- [x] 0.2 Backup current `zabbix_postfix_passive.sh`, `.conf`, and stats/offset files on a test host before migration

## 1. Update passive script

- [x] 1.1 Replace pygtail/pflogsumm discovery chain with fixed defaults: `PYGTAIL=/usr/local/bin/pygtail`, `PFLOGSUMM=/usr/local/bin/pflogsumm`
- [x] 1.2 Add env overrides: `ZABBIX_POSTFIX_PYGTAIL`, `ZABBIX_POSTFIX_PFLOGSUMM`
- [x] 1.3 Rewrite `updatevalue()` to parse `key=value` lines: `grep -m1 "^${key}=" "$TEMPFILE" | cut -d= -f2` (remove k/m/g awk conversion)
- [x] 1.4 Keep update pipeline: `"${PYGTAIL}" -o"${PFOFFSETFILE}" "${MAILLOG}" | "${PFLOGSUMM}" -u 0 --no_bounce_detail ...` (dropped `-h 0`: not accepted by Go binary)
- [x] 1.5 Keep read mode and stats file format (`key;value`) unchanged
- [x] 1.6 Fix typo in error message (`zaabbix` → `zabbix`) while editing
- [x] 1.7 Add startup checks: verify Go binaries exist and are executable before running pipeline

## 2. Update Zabbix config

- [x] 2.1 Change `zabbix_postfix_passive.conf`: `UserParameter=postfix.pfmailq,/usr/local/bin/check_mailq`
- [x] 2.2 Verify `postfix[*]` and `postfix.update_data` lines remain pointing to `sudo /usr/local/sbin/zabbix_postfix_passive.sh`

## 3. Update installer

- [x] 3.1 Replace python3/pip3/pflogsumm/pygtail dependency checks with Go binary checks (`/usr/local/bin/pygtail`, `pflogsumm`, `check_mailq`)
- [x] 3.2 Remove pip install pygtail flow; prompt user to run `make install` from repo if binaries missing
- [x] 3.3 Fix installer typo: `cp .zabbix_postfix_passive.conf` → `cp zabbix_postfix_passive.conf`
- [x] 3.4 Keep sudoers line: `zabbix ALL=(ALL) NOPASSWD: /usr/local/sbin/zabbix_postfix_passive.sh`

## 4. Documentation

- [x] 4.1 Update `README_passive.md`: replace Python/pflogsumm/pip prerequisites with Go binary install steps
- [x] 4.2 Add migration note: offset/stats files from Python/Perl era remain compatible
- [x] 4.3 Document optional env overrides for testing

## 5. Verification

- [x] 5.1 On test host: run `sudo /usr/local/sbin/zabbix_postfix_passive.sh` (update) and verify exit 0 + "OK: statistics updated"
- [x] 5.2 Verify stats file `/tmp/zabbix-postfix-passive-statsfile.dat` contains incremented `key;value` entries
- [x] 5.3 Run `sudo /usr/local/sbin/zabbix_postfix_passive.sh received` and verify integer output
- [x] 5.4 Run `/usr/local/bin/check_mailq` and compare with previous `postfix.pfmailq` grep pipeline on same host
- [x] 5.5 Deploy updated `.conf`, restart zabbix-agent, confirm template items collect data in Zabbix UI
- [x] 5.6 Document rollback steps in README_passive.md (restore old script + conf)
