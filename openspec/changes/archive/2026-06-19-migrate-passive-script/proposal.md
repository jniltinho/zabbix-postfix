## Why

The Go binaries from `converter-to-golang` (`pygtail`, `pflogsumm`, `check_mailq`) replace Python and Perl runtimes, but production Zabbix passive monitoring still runs `zabbix_postfix_passive.sh`, which hardcodes Perl/Python paths and parses human-readable pflogsumm output. This change wires the passive stack to the Go binaries and parses `key=value` metrics, completing the migration started in the previous change.

## What Changes

- Update `zabbix_postfix_passive.sh` to call `/usr/local/bin/pygtail` and `/usr/local/bin/pflogsumm` instead of Python/Perl
- Replace grep/awk parsing of Perl summary text with `key=value` parsing from `pflogsumm-go`
- Update `zabbix_postfix_passive.conf`: `postfix.pfmailq` points to `/usr/local/bin/check_mailq`
- Update `install_postfix_template_zabbix_passive.sh` to check for Go binaries instead of python3/pip/pflogsumm/pygtail
- Update `README_passive.md` with new prerequisites and install steps
- Preserve existing behavior: stats-file accumulation, offset file paths, UserParameter keys, and Zabbix template compatibility

## Capabilities

### New Capabilities

- `passive-script-go`: Updated `zabbix_postfix_passive.sh` orchestrating Go binaries with `key=value` parsing and stats-file accumulation
- `passive-zabbix-config`: Updated UserParameter definitions, sudoers expectations, and installer for Go-based passive mode

### Modified Capabilities

_(none — no main specs exist yet; this change depends on artifacts from `converter-to-golang`)_

## Impact

- **Depends on**: `converter-to-golang` change completed and binaries installed to `/usr/local/bin/`
- **Modified files**: `zabbix-postfix/zabbix_postfix_passive.sh`, `zabbix_postfix_passive.conf`, `install_postfix_template_zabbix_passive.sh`, `README_passive.md`
- **Removed runtime deps on host**: Python 3, pip, pygtail, system pflogsumm (for passive mode)
- **Unchanged**: `template_postfix_passive.xml`, UserParameter key names (`postfix[*]`, `postfix.update_data`, `postfix.pfmailq`), stats/offset file paths under `/tmp/`
- **Sudoers**: still `NOPASSWD: /usr/local/sbin/zabbix_postfix_passive.sh` (script path unchanged; internal binary paths change)
