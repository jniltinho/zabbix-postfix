## ADDED Requirements

### Requirement: Invoke Go binaries for log processing
The passive script SHALL use `/usr/local/bin/pygtail` and `/usr/local/bin/pflogsumm` for the update pipeline. It SHALL NOT invoke Python, pip-installed pygtail, or system Perl pflogsumm.

#### Scenario: Update mode uses Go pipeline
- **WHEN** `zabbix_postfix_passive.sh` is run without arguments (update mode)
- **THEN** it executes `pygtail -o /tmp/zabbix-postfix-passive-offset.dat <mail.log> | pflogsumm` using the Go binaries and exits 0 with "OK: statistics updated"

#### Scenario: Missing Go binary
- **WHEN** `/usr/local/bin/pygtail` or `/usr/local/bin/pflogsumm` is not executable
- **THEN** the script writes an ERROR to stdout and exits 1

#### Scenario: Environment override for testing
- **WHEN** `ZABBIX_POSTFIX_PYGTAIL` or `ZABBIX_POSTFIX_PFLOGSUMM` is set
- **THEN** the script uses those paths instead of the defaults

### Requirement: Parse key=value metrics on update
On update, the script SHALL parse `pflogsumm` default output (`key=value` lines) and accumulate integer values into the stats file. Byte metrics SHALL NOT require k/m/g conversion.

#### Scenario: Accumulate received count
- **WHEN** update runs and pflogsumm outputs `received=5`
- **THEN** the stats file increments the `received` entry by 5

#### Scenario: Byte metrics as integers
- **WHEN** update runs and pflogsumm outputs `bytes_received=125952`
- **THEN** the stats file increments `bytes_received` by 125952 (no suffix parsing)

#### Scenario: All 11 metrics updated
- **WHEN** update completes successfully
- **THEN** all metrics in PFVALS are updated: received, delivered, forwarded, deferred, bounced, rejected, held, discarded, reject_warnings, bytes_received, bytes_delivered

### Requirement: Read mode unchanged for Zabbix
When called with a metric name argument, the script SHALL read the accumulated value from `/tmp/zabbix-postfix-passive-statsfile.dat` and print it to stdout, preserving the existing UserParameter contract.

#### Scenario: Read single metric
- **WHEN** `zabbix_postfix_passive.sh received` is run and stats file contains `received;142`
- **THEN** stdout is `142` and exit code is 0

#### Scenario: Unknown metric
- **WHEN** `zabbix_postfix_passive.sh unknown_metric` is run
- **THEN** stdout contains an ERROR and exit code is 1

### Requirement: Preserve file paths and mail log detection
The script SHALL keep existing paths and mail log auto-detection:
- Offset file: `/tmp/zabbix-postfix-passive-offset.dat`
- Stats file: `/tmp/zabbix-postfix-passive-statsfile.dat`
- Mail log: `/var/log/mail.log` (Debian/Ubuntu) or `/var/log/maillog` (RHEL/CentOS)

#### Scenario: Debian mail log path
- **WHEN** `/var/log/mail.log` exists
- **THEN** the script reads from `/var/log/mail.log`

#### Scenario: RHEL mail log path
- **WHEN** only `/var/log/maillog` exists
- **THEN** the script reads from `/var/log/maillog`

### Requirement: Remove Python/Perl fallback chain
The script SHALL NOT search for `/usr/bin/pygtail`, pip pygtail, or `/usr/sbin/pflogsumm`. Only Go binary paths (with optional env override) are supported.

#### Scenario: No Python fallback
- **WHEN** Go binaries are installed but Python pygtail is also present
- **THEN** the script uses Go binaries only
