## Why

The existing Postfix monitoring stack depends on Python (`pygtail.py`) and Perl (`pflogsumm`, `check_mailq.pl`) runtimes, creating fragile multi-language dependency chains on every monitored host. Converting each tool to a self-contained Go binary eliminates these runtime dependencies, simplifies deployment, and lays the foundation for a native `check_postfix` Zabbix plugin written entirely in Go.

## What Changes

- New Go module `pygtail`: incremental log reader with offset tracking, replacing `pygtail.py`
- New Go module `pflogsumm`: Postfix log summarizer parsing `mail.log`/`maillog`, replacing `/usr/sbin/pflogsumm`
- New Go module `check_mailq`: mail queue depth checker, replacing inline `mailq | grep` pipeline
- Each module is a standalone CLI binary built with Cobra (+ Viper where useful), with a `Makefile` for build/test/install
- Root `README.md` and per-module `README.md` documenting usage, flags, and planned Zabbix integration
- Standard Go `.gitignore` excluding AI tooling dirs (`.claude/`, `.aider*`) and build artifacts
- Test fixtures directory per module (`testdata/`) where real `mail.log` files can be dropped for integration tests

## Capabilities

### New Capabilities

- `pygtail-go`: Incremental log file reader that tracks read offset via an offset file; handles log rotation (copytruncate, dateext, `.1`/`.1.gz`)
- `pflogsumm-go`: Postfix log parser and summarizer producing counts for received, delivered, forwarded, deferred, bounced, rejected, reject_warnings, held, discarded, bytes_received, bytes_delivered
- `check-mailq-go`: Mail queue depth counter using `mailq` output, returning numeric queue size
- `zabbix-compat`: Shared output format and exit-code contract for future Zabbix integration and the `check_postfix` plugin

### Modified Capabilities

_(none — shell scripts and Zabbix `.conf` files are updated in a follow-up change)_

## Impact

- **Shell scripts unchanged in this change**: `zabbix_postfix_passive.sh`, `zabbix_postfix.sh`, and Zabbix `.conf`/XML templates remain as-is until a follow-up change migrates them to call the Go binaries and parse `key=value` output
- **New directories**: `pygtail/`, `pflogsumm/`, `check_mailq/` at repo root, each a self-contained Go module
- **Dependencies added**: Go toolchain (≥1.21); `github.com/spf13/cobra`, `github.com/spf13/viper` per module
- **Deployment (this change)**: build and install Go binaries to `/usr/local/bin/` for manual validation; production Zabbix wiring happens in the follow-up change
- **Follow-up change**: update `zabbix_postfix_passive.sh` to orchestrate `pygtail | pflogsumm`, parse `key=value` metrics, update sudoers and `zabbix_postfix_passive.conf`
- **Future**: exportable parser packages imported by a `check_postfix` Zabbix plugin (active and passive modes)
