# pflogsumm

Go implementation of the Perl pflogsumm Postfix log summarizer. Parses `mail.log` / `maillog` and produces output identical to the Perl version by default. Drop-in replacement — no Perl required on the agent host.

## Prerequisites

- Go ≥ 1.21

## Build

```bash
make build          # produces ./dist/pflogsumm
sudo make install   # installs to /usr/local/bin/pflogsumm
```

## Usage

```
usage: pflogsumm -[eq] [-d <today|yesterday>] [--detail <cnt>]
    [--bounce-detail <cnt>] [--deferral-detail <cnt>]
    [-h <cnt>] [-i|--ignore-case] [--iso-date-time] [--mailq]
    [-m|--uucp-mung] [--no-no-msg-size] [--problems-first]
    [--rej-add-from] [--reject-detail <cnt>] [--smtp-detail <cnt>]
    [--smtpd-stats] [--smtpd-warning-detail <cnt>]
    [--syslog-name=string] [-u <cnt>] [--verbose-msg-detail]
    [--verp-mung[=<n>]] [--zero-fill] [file1 [filen]]

       pflogsumm --[version|help]

Go-specific flags:
  --format string   output format: human, keyvalue, json, summary (default "human")
  --zabbix          output key=value metrics for Zabbix (overrides --format)
  --mailq           append current mail queue after report
```

If no logfile is given, reads from stdin. Multiple files are concatenated.

### Examples

```bash
# Full human-readable report (matches Perl pflogsumm output)
pflogsumm /var/log/mail.log

# Filter to today or yesterday only
pflogsumm -d today /var/log/mail.log
pflogsumm -d yesterday /var/log/mail.log

# Multiple log files (concatenated, same as Perl)
pflogsumm /var/log/mail.log.1 /var/log/mail.log

# Append live mail queue at the end of the report
pflogsumm -d today --mailq /var/log/mail.log

# Zabbix key=value output (for passive monitoring integration)
pflogsumm --zabbix /var/log/mail.log

# Pipe from pygtail (incremental reads — used by Zabbix script)
pygtail -o /tmp/postfix.offset /var/log/mail.log | pflogsumm --zabbix
```

## Output Modes

### Default (human-readable) — identical to Perl pflogsumm

```
Postfix log summaries for Jun 20

Grand Totals
------------
messages

 164181   received
  81697   delivered
      0   forwarded
    407   deferred  (4320  deferrals)
   6656   bounced
 375055   rejected (88%)
...
```

### `--zabbix` — key=value for Zabbix passive monitoring

Integer values, no k/m/g suffixes:

```
received=164181
delivered=81697
forwarded=0
deferred=407
bounced=6656
rejected=375055
reject_warnings=0
held=0
discarded=0
bytes_received=9347000000
bytes_delivered=4820000000
```

## Flag Compatibility with Perl pflogsumm

All flags accepted by the Perl version are accepted here. Most are silently ignored because they control detail sections that the Go version currently does not render. The key flags that ARE implemented:

| Flag | Status | Notes |
|------|--------|-------|
| `[file1 [filen]]` | ✓ implemented | multiple files concatenated |
| `-d today\|yesterday` | ✓ implemented | syslog and RFC3339 timestamps |
| `--mailq` | ✓ implemented | appends `mailq` output |
| `--zabbix` | ✓ implemented (new) | key=value output for Zabbix |
| `-u <cnt>` | ✓ accepted, ignored | |
| `-h <cnt>` | ✓ accepted (`--h`) | cobra reserves `-h` for `--help` |
| `-e / -q / -i / -m` | ✓ accepted, ignored | |
| `--no-bounce-detail` | ✓ accepted, ignored | underscore variant also works |
| `--no-deferral-detail` | ✓ accepted, ignored | |
| `--no-reject-detail` | ✓ accepted, ignored | |
| `--no-smtpd-warnings` | ✓ accepted, ignored | |
| `--no-no-msg-size` | ✓ accepted, ignored | |
| `--detail <cnt>` | ✓ accepted, ignored | |
| `--bounce-detail <cnt>` | ✓ accepted, ignored | |
| `--deferral-detail <cnt>` | ✓ accepted, ignored | |
| `--reject-detail <cnt>` | ✓ accepted, ignored | |
| `--smtp-detail <cnt>` | ✓ accepted, ignored | |
| `--smtpd-warning-detail <cnt>` | ✓ accepted, ignored | |
| `--problems-first` | ✓ accepted, ignored | |
| `--rej-add-from` | ✓ accepted, ignored | |
| `--verbose-msg-detail` | ✓ accepted, ignored | |
| `--zero-fill` | ✓ accepted, ignored | |
| `--iso-date-time` | ✓ accepted, ignored | |
| `--smtpd-stats` | ✓ accepted, ignored | |
| `--verp-mung[=n]` | ✓ accepted, ignored | optional value works |
| `--syslog-name=string` | ✓ accepted, ignored | |

Flag names with underscores (`--no_bounce_detail`) are treated identically to dashes (`--no-bounce-detail`).

## Zabbix Integration

`zabbix_postfix_passive.sh` calls this binary via:

```bash
pygtail -o /tmp/zabbix-postfix-passive-offset.dat /var/log/mail.log \
  | pflogsumm --zabbix -u 0 --no_bounce_detail --no_deferral_detail \
               --no_reject_detail --no_smtpd_warnings --no_no_msg_size
```

`--zabbix` switches output to key=value format consumed by the shell script. The compat flags (`--no_*`) are accepted for script compatibility with the Perl invocation and are silently ignored.

## Testing

```bash
make test          # unit tests (parser + formatter)
```
