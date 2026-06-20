# check_mailq

Counts messages in the Postfix mail queue by running `mailq` and parsing its output.

Works in two modes:

- **Zabbix mode** (`--zabbix`): outputs a raw integer — ideal as a Zabbix `UserParameter` value.
- **Nagios/Icinga mode** (`-w`/`-c`): outputs a status line with perfdata and exits with the
  standard Nagios exit code (0 OK, 1 WARNING, 2 CRITICAL, 3 UNKNOWN).

This is a Go replacement for the Perl `check_mailq` plugin from `nagios-plugins`, supporting
the same flags and output format for the Postfix MTA.

---

## Build and install

```bash
make build          # produces dist/check_mailq
sudo make install   # installs to /opt/zabbix_postfix/check_mailq
```

```bash
make test           # run unit tests
```

---

## Testing with Docker

The `Dockerfile` and `integration-test.sh` in this directory build the Go binary and run
27 tests comparing its output against the Perl `check_mailq` reference implementation
(from the `monitoring-plugins` package, version 2.3.5).

### Run the full test suite

```bash
cd check_mailq
docker build -t check-mailq-test .
docker run --rm check-mailq-test
```

Expected output ends with `All 27 tests passed.`

### Compile and unit-test only (no Docker runtime)

```bash
docker build --target build .
```

This stage runs `go test ./...` and exits. No Ubuntu image or Perl needed.

### What is tested

| # | Scenario | Checked |
|---|----------|---------|
| 1 | Empty queue, `--zabbix` | outputs `0`, exits 0 |
| 2 | Empty queue, Nagios | Perl + Go output `OK`, exits 0 |
| 3 | 3 messages, `-w 5 -c 20` | Perl + Go output `OK: ...below threshold`, exits 0 |
| 4 | 3 messages, `--zabbix` | outputs `3`, exits 0 |
| 5 | 10 messages, `-w 5 -c 20` | Perl + Go output `WARNING`, exits 1 |
| 6 | 10 messages, `--zabbix` | outputs `10`, exits 0 |
| 7 | 25 messages, `-w 5 -c 20` | Perl + Go output `CRITICAL`, exits 2 |
| 8 | Missing `-w`/`-c` | `UNKNOWN`, exits 3 |
| 9 | `-w 20 -c 5` (w ≥ c) | `UNKNOWN`, exits 3 |
| 10 | `-M sendmail` | `UNKNOWN`, exits 3 |
| 11 | `-M postfix` | identical result to no `-M` |
| 12 | `-s` (`--sudo`) | accepted, correct count |
| 13 | `-v` (`--verbose`) | debug lines `Running:` and `msg_q` present |
| 14 | `-t 30` (`--timeout`) | accepted, correct count |
| 15 | `-W`/`-C` domain compat flags | accepted without error, ignored for Postfix |
| 16 | Real postfix mailq binary | graceful error handling when daemon not running |

---

## Modes of operation

### Zabbix UserParameter mode

Use `--zabbix` when calling from `zabbix_postfix_passive.conf`. The binary outputs only the
queue depth as an integer and always exits with code 0. No thresholds are evaluated.

```bash
$ check_mailq --zabbix
51
```

Zabbix configuration:

```
UserParameter=postfix.pfmailq,/opt/zabbix_postfix/check_mailq --zabbix
```

---

### Nagios / Icinga mode

Use `-w` and `-c` to check against thresholds. The binary outputs a status line compatible
with the Nagios plugin standard, including perfdata, and exits with the appropriate code.

```bash
$ check_mailq -w 10 -c 50
OK: postfix mailq (5) is below threshold (10/50)|unsent=5;10;50;0

$ check_mailq -w 10 -c 50
WARNING: postfix mailq is 15 (threshold w = 10)|unsent=15;10;50;0

$ check_mailq -w 10 -c 50
CRITICAL: postfix mailq is 62 (threshold c = 50)|unsent=62;10;50;0
```

Exit codes:

| Code | Meaning |
|------|---------|
| `0` | OK |
| `1` | WARNING — queue size ≥ `-w` |
| `2` | CRITICAL — queue size ≥ `-c` |
| `3` | UNKNOWN — error running `mailq` or missing required flags |

Nagios / Icinga configuration example:

```
define command {
    command_name  check_postfix_mailq
    command_line  /opt/zabbix_postfix/check_mailq -w $ARG1$ -c $ARG2$
}
```

---

## Flag reference

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--warning` | `-w` | int | — | Minimum number of messages in queue to generate a WARNING. Required in Nagios mode. |
| `--critical` | `-c` | int | — | Minimum number of messages to generate a CRITICAL alert. Must be greater than `-w`. Required in Nagios mode. |
| `--Warning` | `-W` | int | — | Minimum messages from/to the same domain to generate WARNING. Accepted for Perl compatibility; ignored for Postfix (sendmail/qmail only). |
| `--Critical` | `-C` | int | — | Minimum messages from/to the same domain to generate CRITICAL. Accepted for Perl compatibility; ignored for Postfix (sendmail/qmail only). |
| `--mailserver` | `-M` | string | autodetect | MTA type: `postfix`, `sendmail`, `qmail`, `exim`, `nullmailer`. Only `postfix` is supported; any other value returns UNKNOWN and exits 3. |
| `--timeout` | `-t` | int (seconds) | `15` | Maximum seconds to wait for `mailq` to respond before aborting with UNKNOWN. |
| `--sudo` | `-s` | bool | false | Prefix the `mailq` invocation with `sudo`. Use when the Zabbix agent user does not have direct access to `mailq`. |
| `--verbose` | `-v` | bool | false | Print debug information to stdout: the full `mailq` output, the parsed count, and the evaluated thresholds. Useful for troubleshooting. |
| `--zabbix` | — | bool | false | Output only the raw integer queue count. Overrides all threshold evaluation. Always exits 0. Use this in Zabbix `UserParameter`. |
| `--mailq-path` | — | string | `mailq` | Path to the `mailq` binary. Override when `mailq` is not in `$PATH`. |
| `--version` | — | bool | — | Print the binary version and exit. |
| `--help` | `-h` | bool | — | Show help and exit. |

---

## Usage examples

```bash
# Zabbix: raw count, no thresholds
check_mailq --zabbix

# Nagios: warn at 10, critical at 50
check_mailq -w 10 -c 50

# Nagios with explicit mailq path
check_mailq -w 10 -c 50 --mailq-path /usr/sbin/mailq

# Nagios with sudo (when zabbix agent user lacks mailq access)
check_mailq -w 10 -c 50 --sudo

# Nagios with 30-second timeout
check_mailq -w 10 -c 50 -t 30

# Debug: see full mailq output and threshold evaluation
check_mailq -w 10 -c 50 --verbose

# Zabbix with custom mailq path and sudo
check_mailq --zabbix --mailq-path /usr/sbin/mailq --sudo
```

---

## Sudo setup

If `mailq` requires elevated privileges on your system, configure sudoers:

```
# /etc/sudoers.d/zabbix-mailq
zabbix ALL=(root) NOPASSWD: /usr/sbin/mailq
```

Then use `--sudo` in the UserParameter:

```
UserParameter=postfix.pfmailq,/opt/zabbix_postfix/check_mailq --zabbix --sudo
```

---

## Parity with the Perl nagios-plugins check_mailq

This binary is a drop-in replacement for the Perl `check_mailq` plugin for Postfix.
It accepts the same flags (`-w`, `-c`, `-W`, `-C`, `-M`, `-t`, `-s`, `-v`) and produces
the same output format and exit codes.

Differences:
- Only the `postfix` MTA is implemented. Passing `-M sendmail` (or any non-postfix MTA) exits UNKNOWN.
- `-W`/`-C` (per-domain thresholds) are accepted but have no effect for Postfix, since `mailq`
  does not expose per-domain breakdowns.
- `--zabbix` is a Go-specific extension that makes the binary suitable as a Zabbix `UserParameter`.
