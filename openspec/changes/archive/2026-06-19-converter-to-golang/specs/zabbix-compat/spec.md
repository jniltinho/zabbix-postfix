## ADDED Requirements

### Requirement: Exit code 0 on success, 1 on error
All binaries SHALL exit 0 on successful execution and 1 on any error condition. Zabbix treats non-zero exit codes from UserParameter scripts as unsupported values.

#### Scenario: Successful run
- **WHEN** a binary completes without error
- **THEN** exit code is 0

#### Scenario: Error run
- **WHEN** a binary encounters an unrecoverable error (missing file, subprocess failure, parse error)
- **THEN** exit code is 1

#### Scenario: Errors go to stderr, not stdout
- **WHEN** any binary encounters a runtime error
- **THEN** the error description is written to stderr and stdout is either empty or contains a neutral value (e.g., `0` for `check_mailq`)

### Requirement: Install path /usr/local/bin/
All binaries SHALL be installed to `/usr/local/bin/` by `make install`. The follow-up passive-script change will update sudoers and `.conf` files to reference this path.

#### Scenario: make install places binary correctly
- **WHEN** `make install` is run from the repo root or any module directory
- **THEN** the binary exists at `/usr/local/bin/<modulename>` and is executable

### Requirement: Version flag on all binaries
Every binary SHALL support `--version` printing `<name> version <semver>` to stdout and exiting 0. This aids debugging in Zabbix deployments.

#### Scenario: Version output
- **WHEN** `pygtail --version` (or `pflogsumm --version` or `check_mailq --version`) is run
- **THEN** stdout is `<name> version X.Y.Z` and exit code is 0

### Requirement: check_mailq stdout contract (direct UserParameter)
When invoked as a standalone Zabbix UserParameter, `check_mailq` SHALL write exactly one integer to stdout followed by a newline, with no labels or extra lines.

#### Scenario: Clean stdout for Zabbix mailq check
- **WHEN** Zabbix agent calls `/usr/local/bin/check_mailq` via `postfix.pfmailq`
- **THEN** stdout contains exactly `<count>\n` and nothing else

### Requirement: pflogsumm stdout contract (piped in passive update)
When invoked in a `pygtail | pflogsumm` pipeline (passive update), `pflogsumm` SHALL write machine-parseable `key=value` lines to stdout (default format). Multi-line output is expected and correct in this context.

#### Scenario: Pipeline output parseable by future passive script
- **WHEN** `pygtail -o /tmp/offset.dat /var/log/mail.log | pflogsumm` is run
- **THEN** stdout contains one `key=value` line per metric, parseable with `grep '^received=' | cut -d= -f2`

### Requirement: pygtail stdout contract (piped in passive update)
When invoked in a pipeline, `pygtail` SHALL write only unread log lines to stdout (raw Postfix log lines). Errors go to stderr.

#### Scenario: Pipeline emits log lines only
- **WHEN** `pygtail -o /tmp/offset.dat /var/log/mail.log | pflogsumm` is run
- **THEN** pygtail stdout contains only Postfix log lines (no metrics, no labels)

### Requirement: Zabbix integration deferred to follow-up change
This change SHALL NOT modify `zabbix_postfix_passive.sh`, `zabbix_postfix_passive.conf`, sudoers, or XML templates. The follow-up change will wire UserParameters to the Go binaries.

#### Scenario: Binaries validated before Zabbix wiring
- **WHEN** Go binaries pass unit, golden, and integration tests
- **THEN** they are ready for the follow-up change to update `zabbix_postfix_passive.sh` to call `/usr/local/bin/pygtail`, `/usr/local/bin/pflogsumm`, and `/usr/local/bin/check_mailq`

#### Scenario: Planned passive UserParameters (reference only, not implemented here)
- **WHEN** the follow-up change is applied
- **THEN** `zabbix_postfix_passive.conf` is expected to define something like:
  ```
  UserParameter=postfix.pfmailq,/usr/local/bin/check_mailq
  UserParameter=postfix[*],sudo /usr/local/sbin/zabbix_postfix_passive.sh $1
  UserParameter=postfix.update_data,sudo /usr/local/sbin/zabbix_postfix_passive.sh
  ```
  where the updated passive script orchestrates `pygtail | pflogsumm` internally and serves single-metric reads from the stats file
