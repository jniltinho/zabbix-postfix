# check_mailq

Counts messages in the Postfix mail queue by running `mailq` and parsing its output. Output is a single integer — suitable as a Zabbix UserParameter value.

## Prerequisites

- Go ≥ 1.21
- `mailq` in PATH (or specify path with `--mailq-path`)

## Build

```bash
make build          # produces ./check_mailq
sudo make install   # installs to /usr/local/bin/check_mailq
```

## Usage

```
check_mailq [flags]

Flags:
      --mailq-path string   path to the mailq binary (default "mailq")
      --timeout duration    subprocess timeout (default 10s)
      --version             print version and exit
  -h, --help                show this help
```

### Example

```bash
$ check_mailq
5

$ check_mailq --mailq-path /usr/sbin/mailq
5
```

## Parity with grep pipeline

`check_mailq` matches the output of:

```bash
mailq | grep -v "Mail queue is empty" | grep -c '^[0-9A-Z]'
```

Both count lines beginning with a hex queue ID (digit or uppercase letter).

## Zabbix UserParameter

`zabbix_postfix_passive.conf` defines:

```
UserParameter=postfix.pfmailq,/usr/local/bin/check_mailq
```

`mailq` does not require elevated privileges on most setups, so no sudoers entry is needed for this binary.

## Testing

```bash
make test   # unit tests with fixture files in testdata/
```
