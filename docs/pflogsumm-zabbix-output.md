# pflogsumm --zabbix output explained

The `--zabbix` flag makes pflogsumm emit a flat `key=value` format instead of the
human-readable report. Each line is one metric — no headers, no units, no tables.
This makes it trivial to parse inside a shell script or Zabbix UserParameter.

## Example output

```
# Full log (all time)
$ pflogsumm --zabbix /var/log/mail.log
received=167342
delivered=82608
forwarded=0
deferred=411
bounced=6825
rejected=378990
reject_warnings=0
held=0
discarded=0
bytes_received=9846038973
bytes_delivered=12088619184

# Today only
$ pflogsumm --zabbix -d today /var/log/mail.log
received=5468
delivered=2579
forwarded=0
deferred=59
bounced=244
rejected=7081
reject_warnings=0
held=0
discarded=0
bytes_received=94484809
bytes_delivered=94779169
```

## What each field means

| Key | Unit | Description |
|-----|------|-------------|
| `received` | messages | Total messages accepted by Postfix (SMTP IN + local submission). |
| `delivered` | messages | Messages successfully delivered to a final destination (local mailbox, remote SMTP relay, pipe, etc.). One message sent to 3 recipients counts as 3 deliveries. |
| `forwarded` | messages | Messages forwarded via `.forward` or alias expansion to an external address. |
| `deferred` | messages | Delivery attempts that failed temporarily and were re-queued. A single message can generate multiple deferral events; this counter reflects the total number of deferral events, not unique messages. |
| `bounced` | messages | Messages permanently rejected by the destination and returned to the sender (NDR / bounce). |
| `rejected` | messages | Messages rejected at the SMTP level — before Postfix ever accepted them into the queue. Includes spam rejections, RBL hits, policy checks, etc. |
| `reject_warnings` | messages | Messages that *would* have been rejected under the current policy but were accepted with a warning (warn_if_reject). |
| `held` | messages | Messages placed on hold in the deferred queue by an administrator (`postsuper -h`). |
| `discarded` | messages | Messages silently dropped by a `discard` action in a Postfix policy or header/body check. |
| `bytes_received` | bytes | Total size of all received messages. |
| `bytes_delivered` | bytes | Total size of all delivered messages. Can exceed `bytes_received` when one message is delivered to multiple recipients. |

## Reading the numbers

### received vs delivered

`delivered` can be lower than `received` when messages are still in the queue,
were bounced, or were rejected after acceptance. It can also be higher when the
same message is delivered to multiple recipients (each recipient counts as one
delivery).

```
received=5468   ← 5 468 messages entered Postfix today
delivered=2579  ← 2 579 recipient deliveries completed
```

The gap (5468 − 2579 = 2889) is explained by bounced (244) + still in queue +
rejected after DATA + multi-recipient counting differences.

### rejected vs received

```
rejected=7081   ← rejected at SMTP level (never entered the queue)
received=5468   ← accepted into the queue
```

Rejected messages are stopped *before* queuing, so `rejected` is not subtracted
from `received`. A server rejecting more than it receives is normal for a
mail server under heavy spam pressure.

### bytes_received vs bytes_delivered

```
bytes_received=94484809    ≈  90 MB
bytes_delivered=94779169   ≈  90 MB
```

When `bytes_delivered > bytes_received` it means some messages were delivered to
more than one recipient (each delivery copies the full message body).

### deferred

```
deferred=59
```

Transient failures: the remote server was unreachable, returned a 4xx code, or
Postfix hit a rate limit. Postfix will retry automatically. A persistently high
deferred count suggests connectivity issues, greylisting, or a backlog.

### bounced

```
bounced=244
```

Hard failures: invalid recipient addresses, relay denials, or policy rejections
*after* the message was accepted. Each bounce generates a DSN back to the sender.

## `-d today` vs no date filter

| Invocation | Scope |
|-----------|-------|
| `pflogsumm --zabbix /var/log/mail.log` | All entries present in the log file (days or weeks, depending on rotation) |
| `pflogsumm --zabbix -d today /var/log/mail.log` | Only log lines timestamped with today's date |
| `pflogsumm --zabbix -d yesterday /var/log/mail.log` | Only log lines timestamped with yesterday's date |

In the Zabbix passive agent workflow (`zabbix_postfix_passive.sh`) neither flag is
used. Instead, `pygtail` pipes only the *new* (unread) lines to pflogsumm on each
run, so the date filter is not needed — the time window is controlled by how
frequently Zabbix calls the script.

---

## Flag reference

### Output format flags

#### `--zabbix`
Emit flat `key=value` pairs (one metric per line). Overrides `--format`.
Designed for Zabbix `UserParameter` consumption.

```bash
pflogsumm --zabbix /var/log/mail.log
```

#### `--format string`
*(Go-specific)* Select the output format. Valid values:

| Value | Description |
|-------|-------------|
| `human` | Human-readable report (default) |
| `keyvalue` | Same as `--zabbix` — flat `key=value` pairs |
| `json` | JSON object with all metrics |
| `summary` | Condensed one-screen summary |

```bash
pflogsumm --format json /var/log/mail.log
pflogsumm --format summary /var/log/mail.log
```

---

### Date filtering flags

#### `-d today` / `-d yesterday`
Restrict analysis to log lines timestamped with today's or yesterday's date.
Useful when running pflogsumm directly on a log file that spans multiple days.

```bash
pflogsumm --zabbix -d today /var/log/mail.log
pflogsumm --zabbix -d yesterday /var/log/mail.log
```

> Not used in the Zabbix passive agent — `pygtail` controls the time window instead.

---

### Detail level flags

These flags control how many entries are shown in the detail sections of the
human-readable report. The value is the maximum number of entries to print;
`0` means unlimited, `-1` (Perl default) also means unlimited.

#### `--detail <cnt>`
Master detail level — sets the default cap for all detail sections that do not
have their own flag. Individual flags below override this.

```bash
pflogsumm --detail 20 /var/log/mail.log   # show at most 20 entries per section
pflogsumm --detail 0  /var/log/mail.log   # unlimited
```

#### `--bounce-detail <cnt>`
Maximum entries in the **message bounce detail** section (bounced messages
grouped by relay). Suppressed entirely with `--no_bounce_detail`.

#### `--deferral-detail <cnt>`
Maximum entries in the **message deferral detail** section (deferred messages
grouped by smtp process and reason). Suppressed entirely with `--no_deferral_detail`.

#### `--reject-detail <cnt>` / `--smtp-detail <cnt>`
Maximum entries in the **message reject detail** section.
`--smtp-detail` specifically caps the smtp client reject entries.

#### `--smtpd-warning-detail <cnt>`
Maximum entries in the **smtpd warnings** section.

#### `-h <cnt>`
Maximum number of entries in the **top hosts/domains** tables (senders,
recipients, etc.). Default is unlimited.

#### `-u <cnt>`
Maximum number of entries in the **top users** tables. In the Zabbix passive
script this is set to `0` (unlimited) because the `--zabbix` output does not
include per-user tables anyway.

```bash
# Used in zabbix_postfix_passive.sh:
pflogsumm --zabbix -u 0 ...
```

---

### Suppression flags

These flags remove entire sections from the human-readable report. Useful when
piping to `--zabbix` or `--format keyvalue` where sections are irrelevant, or
when a section is too noisy.

| Flag | Section suppressed |
|------|--------------------|
| `--no_bounce_detail` | Message bounce detail |
| `--no_deferral_detail` | Message deferral detail |
| `--no_reject_detail` | Message reject detail |
| `--no_smtpd_warnings` | smtpd warning detail |
| `--no-no-msg-size` | "Messages with no size data" section |

All four suppression flags are used together in `zabbix_postfix_passive.sh`
because the `--zabbix` output only needs the aggregate counters:

```bash
pflogsumm --zabbix -u 0 \
  --no_bounce_detail --no_deferral_detail \
  --no_reject_detail --no_smtpd_warnings --no_no_msg_size
```

---

### Behaviour flags

#### `-e`
*(extended)* Include the **per-day traffic summary** and **per-hour average**
sections in the human-readable report. Off by default.

#### `-q`
*(quiet)* Suppress the "no statistics for <period>" warning when the log
contains no entries matching the selected date range.

#### `-i` / `--ignore-case`
Treat sender and recipient addresses as case-insensitive. Useful when your MTA
has mixed-case addresses in the log.

#### `--iso-date-time`
Print timestamps in ISO 8601 format (`YYYY-MM-DD HH:MM`) instead of the
default Postfix `Mon DD HH:MM` format.

#### `--problems-first`
Reorder the report so that the **Problems** section (bounced, deferred,
rejected) appears before the **Top senders/recipients** tables. Useful for
quick triage.

#### `--rej-add-from`
Include the sender address in the reject detail entries. By default only the
recipient or reason is shown.

#### `--smtpd-stats`
Include per-domain smtpd connection statistics in the report.

#### `-m` / `--uucp-mung`
Strip UUCP-style `!`-separated bang paths from addresses. Rarely needed on
modern mail servers.

#### `--syslog-name=string`
The syslog program name to look for in the log lines (default: `postfix`).
Override if you run multiple Postfix instances with distinct `syslog_name`
values in `main.cf`.

```bash
pflogsumm --syslog-name=postfix-outbound /var/log/mail.log
```

#### `--verbose-msg-detail`
Show the full message-ID and headers in the message detail sections instead
of the truncated subject line.

#### `--verp-mung[=<n>]`
Collapse VERP-encoded bounce addresses (e.g. `list-owner+user=domain@list.tld`)
into their base form. Without `=<n>` collapses completely; with `=2` keeps two
address components.

#### `--zero-fill`
In the per-hour and per-day tables, print a `0` for hours/days with no
traffic instead of leaving them blank. Makes the tables easier to read in
spreadsheets.

---

### Queue flag

#### `--mailq`
*(Go-specific)* Append the output of `mailq` (current Postfix queue) at the
end of the report. Requires `mailq` to be in `$PATH` and the user to have
permission to run it.

```bash
pflogsumm --format summary --mailq /var/log/mail.log
```

---

### Informational flags

#### `--version`
Print the pflogsumm version and exit.

#### `--help` / `-h` (no argument)
Print the usage summary and exit.
