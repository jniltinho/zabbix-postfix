# pygtail

Go implementation of [pygtail](https://pypi.org/project/pygtail/) — prints only new lines from a log file since the last run. Drop-in replacement for `pygtail.py` bundled in this repository.

## Prerequisites

- Go ≥ 1.26.4

## Build

```bash
make build          # produces ./pygtail
sudo make install   # installs to /usr/local/bin/pygtail
```

## Usage

```
pygtail [flags] <logfile>

Flags:
  -o, --offset-file string   offset file path (default: <logfile>.offset)
      --no-copytruncate       disable copytruncate support
      --version               print version and exit
  -h, --help                  show this help
```

## Flag Comparison with Python pygtail

| Python flag | Go support | Notes |
|-------------|-----------|-------|
| `-o / --offset-file` | ✓ identical | same format, offset-compatible |
| `--no-copytruncate` | ✓ identical | same behaviour |
| `-p / --paranoid` | ✗ not implemented | flushes offset after every line; not needed for Zabbix use |
| `-n / --every-n` | ✗ not implemented | flushes offset every N lines; not needed for Zabbix use |

The shell script (`zabbix_postfix_passive.sh`) uses only `-o`, so the missing flags have no impact on the Zabbix integration.

### Examples

```bash
# Print new lines since last run, save offset to /var/log/mail.log.offset
pygtail /var/log/mail.log

# Use a custom offset file
pygtail -o /tmp/postfix.offset /var/log/mail.log

# Pipe into pflogsumm (replaces: pygtail.py -o /tmp/p.offset /var/log/mail.log | pflogsumm)
pygtail -o /tmp/postfix.offset /var/log/mail.log | pflogsumm
```

## Offset File Format

Two-line text file: inode number on line 1, byte offset on line 2. **Identical to pygtail.py v0.11.1** — the Go binary continues where the Python script left off with no data loss.

## Log Rotation

Detection priority (same as pygtail.py):
1. `<logfile>.0` — savelog
2. `<logfile>.1` — logrotate with `delaycompress`
3. `<logfile>.1.gz` — logrotate without `delaycompress`
4. Dateext patterns: `<logfile>-YYYYMMDD`, `<logfile>-YYYYMMDD.gz`, and epoch-suffixed variants
5. `<logfile>.YYYY-MM-DD` — Python `TimedRotatingFileHandler`

Compressed `.gz` files are transparently decompressed.

**Copytruncate**: enabled by default. When the same inode shrinks below the stored offset, reading restarts from the beginning. Disable with `--no-copytruncate` to get a stderr warning instead.

## Testing

```bash
make test                                          # unit tests (no fixtures needed)
make -C .. fetch-testdata HOST=mx01                # fetch real mail.log
go test -tags integration ./...                    # integration tests
```

## Zabbix Integration

`zabbix_postfix_passive.sh` calls this binary via:

```bash
pygtail -o /tmp/zabbix-postfix-passive-offset.dat /var/log/mail.log | pflogsumm ...
```

The offset file is shared with the old `pygtail.py` — migration requires no reset.
