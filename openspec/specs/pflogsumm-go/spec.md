## ADDED Requirements

### Requirement: Parse Postfix mail.log and produce metric counts
The system SHALL parse Postfix `mail.log` (Debian/Ubuntu) or `maillog` (RHEL/CentOS) format from stdin or a file argument, and produce counts for the following metrics:
`received`, `delivered`, `forwarded`, `deferred`, `bounced`, `rejected`, `reject_warnings`, `held`, `discarded`, `bytes_received`, `bytes_delivered`.

Byte metrics SHALL be returned as integer bytes (no k/m/g suffix).

#### Scenario: Parse from stdin
- **WHEN** `cat mail.log | pflogsumm` is run
- **THEN** metric counts are written to stdout, one per line, as `<key>=<value>`

#### Scenario: Parse from file argument
- **WHEN** `pflogsumm /var/log/mail.log` is run
- **THEN** metric counts are written to stdout in the same format as stdin

#### Scenario: Byte metrics returned as integers
- **WHEN** pflogsumm parses a log containing "123k bytes received"
- **THEN** `bytes_received=125952` is output (123 × 1024)

#### Scenario: Empty log produces zero counts
- **WHEN** an empty or zero-activity log is parsed
- **THEN** all metrics are output with value 0

### Requirement: Key=value output format (default)
The default output SHALL be one metric per line in `<key>=<value>` format, e.g.:
```
received=142
delivered=139
forwarded=0
deferred=3
bounced=1
rejected=5
reject_warnings=0
held=0
discarded=0
bytes_received=1048576
bytes_delivered=1044480
```

#### Scenario: Default output format
- **WHEN** `pflogsumm mail.log` is run without `--format`
- **THEN** stdout matches the `key=value` format with all 11 metrics present

### Requirement: JSON output format via --format flag
The system SHALL support `--format json` flag outputting a single JSON object with all metric keys.

#### Scenario: JSON output
- **WHEN** `pflogsumm --format json mail.log` is run
- **THEN** stdout is a valid JSON object with all 11 metric keys and integer values

### Requirement: Human-readable summary via --format summary
The system SHALL support `--format summary` producing output visually similar to the original Perl pflogsumm for human review and debugging.

#### Scenario: Summary format header
- **WHEN** `pflogsumm --format summary mail.log` is run
- **THEN** stdout includes a header line and indented counts labeled in plain English

### Requirement: Suppress detail sections (compatibility flags)
The system SHALL accept compatibility flags from both passive and active shell scripts, even as no-ops:
- Passive style: `--no_bounce_detail`, `--no_deferral_detail`, `--no_reject_detail`, `--no_smtpd_warnings`, `--no_no_msg_size`
- Active style: `--bounce_detail=0`, `--deferral_detail=0`, `--reject_detail=0`, `--smtpd_warning_detail=0`, `--no_no_msg_size`
- Common: `-h 0`, `-u 0`

#### Scenario: Passive compatibility flags accepted without error
- **WHEN** `pflogsumm -h 0 -u 0 --no_bounce_detail --no_deferral_detail --no_reject_detail --no_smtpd_warnings --no_no_msg_size mail.log` is run
- **THEN** the command exits 0 and produces metric output (flags are silently accepted)

#### Scenario: Active compatibility flags accepted without error
- **WHEN** `pflogsumm -h 0 -u 0 --bounce_detail=0 --deferral_detail=0 --reject_detail=0 --no_no_msg_size --smtpd_warning_detail=0 mail.log` is run
- **THEN** the command exits 0 and produces metric output (flags are silently accepted)

### Requirement: Graceful handling of unreadable input
The system SHALL exit with code 1 and write a descriptive error to stderr if the input file does not exist. When no file argument is given, it SHALL read from stdin (empty stdin produces zero counts, not an error).

#### Scenario: File not found
- **WHEN** `pflogsumm /nonexistent/mail.log` is run
- **THEN** stderr contains an error, stdout is empty, exit code is 1

### Requirement: Golden test parity with bundled pflogsumm.pl
Metric counts produced by `pflogsumm-go` SHALL match bundled `zabbix-postfix/pflogsumm.pl` output for the same log input on the 11 Zabbix metrics, within integration/golden test fixtures.

#### Scenario: Golden test on sample log
- **WHEN** the same `testdata/mail.log` slice is parsed by Go and Perl
- **THEN** all 11 metric values match

### Requirement: Module structure and exportable parser
All exported types and functions SHALL have godoc comments. The module SHALL expose a public parser at `pflogsumm/pkg/parser` with `Parse(r io.Reader) (Metrics, error)`, enabling future import by the `check_postfix` plugin. CLI wiring lives in `pflogsumm/main.go` and `pflogsumm/cmd/`.

#### Scenario: Parser package importable by future plugin
- **WHEN** another Go module imports `pflogsumm/pkg/parser`
- **THEN** it can call `parser.Parse(r io.Reader) (Metrics, error)` to get metric counts without invoking the CLI
