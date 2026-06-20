# Zabbix API Setup

This guide covers the Zabbix JSON-RPC API operations used in this project.
All steps can be run individually or automated with the scripts in `scripts/`.

Tested on Zabbix **7.0.x**.

---

## Prerequisites

- `python3` available locally
- A Zabbix user with **Super Admin** profile
- API endpoint: `http://<zabbix-server>/api_jsonrpc.php`

---

## 1. Generate an API token

API tokens are long-lived credentials that do not expire with the session.
There are two ways to get one.

### Option A — Script (recommended)

```bash
python3 scripts/zabbix-create-token.py \
  -u http://<zabbix-server> \
  -U Admin \
  -p <password> \
  --name "zabbix-postfix" \
  --expires 365
```

Output:

```
[OK]  Login successful
[..] Token will expire on 2027-06-20 (365 days)

[OK]  API token created: 'zabbix-postfix'

  Token: abc123...

Save this token — it will not be shown again.
```

Options:

| Flag | Default | Description |
|------|---------|-------------|
| `-u` | — | Zabbix server URL |
| `-U` | `Admin` | Zabbix username |
| `-p` | — | Zabbix password |
| `--name` | `zabbix-postfix` | Token name |
| `--expires` | `0` (never) | Expiry in days |
| `--print-only` | — | Print token only (for shell capture) |

### Option B — Zabbix web interface

1. Log in → click your username (top-right) → **User settings → API tokens**
2. Click **Create API token**, set a name and expiry → **Add**
3. Copy the token — it is shown only once

---

## 2. Automated setup — `scripts/zabbix-api-setup.py`

The script [`scripts/zabbix-api-setup.py`](../scripts/zabbix-api-setup.py) automates
all configuration steps in a single command using the API token.

### Full setup

```bash
python3 scripts/zabbix-api-setup.py \
  -u http://<zabbix-server> \
  -t <API_TOKEN> \
  -H 10683,10684,10685,10686,10687,10688,10689
```

Steps performed in order:

| # | Step | Description |
|---|------|-------------|
| 1 | `--import` | Imports `template_postfix_passive.xml` into Zabbix |
| 2 | `--link` | Links the template to the specified hosts |
| 3 | `--macros` | Creates threshold macros on each host (2× template defaults) |
| 4 | `--group` | Creates the `MAIL_SERVERS` host group and adds the hosts |
| 5 | `--script` | Creates the **Reset Postfix offset** remote script |

### Run individual steps

```bash
# Import template only
python3 scripts/zabbix-api-setup.py -u http://<zabbix-server> -t <TOKEN> --import

# Link template to hosts
python3 scripts/zabbix-api-setup.py -u http://<zabbix-server> -t <TOKEN> -H 10683,10684 --link

# Create threshold macros
python3 scripts/zabbix-api-setup.py -u http://<zabbix-server> -t <TOKEN> -H 10683,10684 --macros

# Create MAIL_SERVERS group and add hosts
python3 scripts/zabbix-api-setup.py -u http://<zabbix-server> -t <TOKEN> -H 10683,10684 --group

# Create the Reset Postfix offset script
python3 scripts/zabbix-api-setup.py -u http://<zabbix-server> -t <TOKEN> --script
```

### Environment variables

```bash
export ZABBIX_URL=http://<zabbix-server>
export ZABBIX_TOKEN=<token>
export ZABBIX_HOSTIDS=10683,10684,10685,10686,10687,10688,10689

python3 scripts/zabbix-api-setup.py
```

| Flag | Environment variable |
|------|----------------------|
| `-u` | `ZABBIX_URL` |
| `-t` | `ZABBIX_TOKEN` |
| `-H` | `ZABBIX_HOSTIDS` |

---

## 3. Template import

The `--import` step reads `template_postfix_passive.xml` directly from the
repository root and sends it to the Zabbix API.

**Zabbix 7.0 format requirements** (already handled by the script):

- The `<date>` tag is stripped before sending — Zabbix 7.0 rejects it.
- The `<graphs>` element must be at the root of `<zabbix_export>`, **outside**
  the `<template>` element. The template in this project is already correct.

After import, confirm the template exists:

```bash
curl -s ${ZABBIX_URL}/api_jsonrpc.php \
  -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"template.get\",
       \"params\":{\"output\":[\"templateid\",\"name\"],
       \"filter\":{\"name\":\"Template App Postfix by Zabbix agent (passive)\"}},
       \"auth\":\"${ZABBIX_TOKEN}\",\"id\":1}" \
  | python3 -m json.tool
```

---

## 4. Threshold macros

The template defines three macros in its triggers. Each host can override the
value without needing a separate template.

| Macro | Template default | Script default (2×) |
|-------|-----------------|----------------------|
| `{$POSTFIX_MAILQ_WARN}` | 100 | 200 |
| `{$POSTFIX_DEFERRED_WARN}` | 50 | 100 |
| `{$POSTFIX_REJECTED_WARN}` | 1000 | 2000 |

Triggers use `avg(...,5m)` — a problem fires only if the **5-minute average**
exceeds the threshold, avoiding alerts from momentary spikes.

To override a macro on a specific host after setup:
**Configuration → Hosts → (host) → Macros → Add**

---

## 5. Reset Postfix offset script

The **Reset Postfix offset** script is created by the `--script` step of
`scripts/zabbix-api-setup.py`. It calls `sudo /opt/zabbix_postfix/zabbix-reset-offset.sh`
on the Zabbix agent, resetting the `pygtail` offset to the end of `mail.log`
and clearing the stats file.

**What it does:**
1. Reads the current inode and size of `/var/log/mail.log`
2. Writes a new offset file pointing to the end of the log
3. Deletes the stats file so counters restart from zero

**When to use:** Postfix metric counters spike on first collection because
`pygtail` reads the entire existing log on its first run. Run this script to
reset the baseline so only new log entries are counted going forward.

### Prerequisites on each mail server

**1. Deploy the reset script**

Installed automatically by the `.deb`/`.rpm` package at
`/opt/zabbix_postfix/zabbix-reset-offset.sh`.

If installing manually from the tarball, copy and set permissions:

```bash
scp scripts/zabbix-reset-offset.sh mailserver:/opt/zabbix_postfix/
ssh mailserver chmod +x /opt/zabbix_postfix/zabbix-reset-offset.sh
```

**2. Add sudoers entry**

Added automatically by the `.deb`/`.rpm` `postinst` script.

The `/tmp/zabbix-postfix-passive-*.dat` files are owned by `root` (written by
the passive script which runs via `sudo`). The reset script must also run as root.
If installing manually, add to `/etc/sudoers`:

```
zabbix ALL=(ALL) NOPASSWD: /opt/zabbix_postfix/zabbix-reset-offset.sh
```

Without this entry the script fails with `Permission denied` on the offset and
stats files.

**3. Enable `system.run` on the Zabbix agent**

Add to `/etc/zabbix/zabbix_agent2.conf`:

```
AllowKey=system.run[*]
```

Then restart the agent:

```bash
systemctl restart zabbix-agent2
```

### How to run

Monitoring → Hosts → click a mail server → **Scripts → Reset Postfix offset**

Expected output:
```
Reset OK: inode=263421 offset=477237894
```

After running, counters restart from zero. The `avg(5m)` trigger stabilizes
within 5 minutes and false alarms stop.

---

## 6. Find host IDs

```bash
curl -s ${ZABBIX_URL}/api_jsonrpc.php \
  -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"host.get\",
       \"params\":{\"output\":[\"hostid\",\"name\"]},
       \"auth\":\"${ZABBIX_TOKEN}\",\"id\":1}" \
  | python3 -c "
import sys, json
for h in json.load(sys.stdin)['result']:
    print(h['hostid'], h['name'])
"
```
