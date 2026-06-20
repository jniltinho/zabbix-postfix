# pflogsumm

Go implementation of the Postfix log summarizer. Parses `mail.log` / `maillog` and outputs metric counts. Drop-in replacement for the Perl `pflogsumm` tool used by the Zabbix monitoring scripts.

## Prerequisites

- Go ≥ 1.21

## Build

```bash
make build          # produces ./pflogsumm
sudo make install   # installs to /usr/local/bin/pflogsumm
```

## Usage

```
pflogsumm [flags] [logfile]

If no logfile is given, reads from stdin.

Flags:
      --format string   output format: keyvalue, json, summary (default "keyvalue")
  -v, --version         print version and exit
  -h, --help            show this help
```

### Examples

```bash
# Parse a log file (default key=value output)
pflogsumm /var/log/mail.log

# Pipe from pygtail (incremental)
pygtail -o /tmp/postfix.offset /var/log/mail.log | pflogsumm

# JSON output for scripting
pflogsumm --format json /var/log/mail.log

# Human-readable summary (similar to Perl pflogsumm)
pflogsumm --format summary /var/log/mail.log

# Extract a single metric in shell
pflogsumm /var/log/mail.log | grep '^received=' | cut -d= -f2
```

## Output Reference

Default (`keyvalue`) format — one metric per line:

| Key | Description |
|-----|-------------|
| `received` | Messages accepted by the MTA (smtpd client= + pickup) |
| `delivered` | Messages successfully delivered |
| `forwarded` | Messages forwarded |
| `deferred` | Unique messages with at least one deferral |
| `bounced` | Messages bounced (status=bounced) |
| `rejected` | Messages rejected (NOQUEUE: reject + cleanup milter-reject) |
| `reject_warnings` | Reject warnings |
| `held` | Messages held |
| `discarded` | Messages discarded |
| `bytes_received` | Total bytes received (integer bytes) |
| `bytes_delivered` | Total bytes delivered (integer bytes, counted per recipient) |

## Flag Comparison with Perl pflogsumm

| Perl flag | Go support | Notes |
|-----------|-----------|-------|
| `[logfile]` | ✓ | positional arg or stdin |
| `--format` | ✓ new | `keyvalue` (default), `json`, `summary` |
| `-u <cnt>` | ✓ accepted, ignored | top-users count; irrelevant (no detail sections) |
| `-h <cnt>` | ✗ not accepted | cobra reserves `-h` for `--help`; drop from scripts |
| `--no_bounce_detail` | ✓ accepted, ignored | no detail sections in Go version |
| `--no_deferral_detail` | ✓ accepted, ignored | |
| `--no_reject_detail` | ✓ accepted, ignored | |
| `--no_smtpd_warnings` | ✓ accepted, ignored | |
| `--no_no_msg_size` | ✓ accepted, ignored | |
| `--bounce-detail <cnt>` | ✓ accepted, ignored | |
| `--deferral-detail <cnt>` | ✓ accepted, ignored | |
| `--reject-detail <cnt>` | ✓ accepted, ignored | |
| `--smtp-detail <cnt>` | ✓ accepted, ignored | |
| `--smtpd-warning-detail <cnt>` | ✓ accepted, ignored | |
| `--detail <cnt>` | ✓ accepted, ignored | |
| `--problems-first` | ✓ accepted, ignored | |
| `--rej-add-from` | ✓ accepted, ignored | |
| `--ignore-case` | ✓ accepted, ignored | |
| `--verbose-msg-detail` | ✓ accepted, ignored | |
| `--zero-fill` | ✓ accepted, ignored | |
| `--iso-date-time` | ✓ accepted, ignored | |
| `--smtpd-stats` | ✓ accepted, ignored | |
| `--mailq` | ✓ accepted, ignored | use `check_mailq` binary instead |
| `--syslog-name=string` | ✓ accepted, ignored | |
| `-d <today\|yesterday>` | ✓ accepted, ignored | no date filtering in Go version |
| `-e / --extended` | not registered | not needed |
| `-q` | not registered | not needed |
| `--verp-mung` | not registered | not needed |

## Output Format Compatibility

The Perl pflogsumm outputs a human-readable summary:
```
 141741   received
  51313   delivered
   7231m  bytes received
```

The Go pflogsumm defaults to `key=value` (integer bytes, no k/m/g suffixes):
```
received=141741
delivered=51313
bytes_received=7593259799
```

`zabbix_postfix_passive.sh` was updated to parse `key=value` output and no longer calls Perl pflogsumm. After migration, `apt install pflogsumm` is no longer required on the agent host.

## Testing

```bash
make test                               # unit tests
make -C .. fetch-testdata HOST=mx01    # fetch real mail.log from mx01
go test -tags integration ./...        # golden test vs pflogsumm.pl
```

The golden test (`golden_test.go`) compares Go vs Perl pflogsumm on the same `testdata/mail.log`. All 5 key metrics match exactly when the log is not actively growing.

## Zabbix Integration

`zabbix_postfix_passive.sh` calls this binary via:

```bash
pygtail -o /tmp/zabbix-postfix-passive-offset.dat /var/log/mail.log \
  | pflogsumm -u 0 --no_bounce_detail --no_deferral_detail \
               --no_reject_detail --no_smtpd_warnings --no_no_msg_size
```

The `-h 0` flag (used by the old Perl invocation) is **not accepted** — it is not registered in the Go binary because cobra reserves `-h` for `--help`. The script drops it.
