# HOWTO — zabbix-postfix

Complete installation and configuration guide for Postfix monitoring with Zabbix using Go binaries.

## Overview

```
Zabbix Server
    │
    │  polls every 1–3 min
    ▼
Zabbix Agent 2 (on mail server)
    │
    ├─ postfix.update_data ──▶  zabbix_postfix_passive.sh
    │                              │
    │                              ├─ pygtail   (incremental log reader)
    │                              ├─ pflogsumm (Postfix log parser)
    │                              └─ /tmp/zabbix-postfix-passive-statsfile.dat
    │
    ├─ postfix[received]   ──▶  reads statsfile → integer
    ├─ postfix[delivered]  ──▶  reads statsfile → integer
    ├─ ...                 ──▶  reads statsfile → integer
    │
    └─ postfix.pfmailq     ──▶  check_mailq → integer (live queue depth)
```

All three tools (`pygtail`, `pflogsumm`, `check_mailq`) are Go binaries compiled from this repository. No Python, no Perl, no `apt install pflogsumm` required on the mail server.

---

## Prerequisites

### Build machine (where you compile)

- Go ≥ 1.21
- `make`
- `upx` (for binary compression — `apt install upx` or `yum install upx`)

### Mail server (Zabbix agent host)

- Postfix running and writing to `/var/log/mail.log` (Debian/Ubuntu) or `/var/log/maillog` (RHEL/CentOS)
- `zabbix-agent` or `zabbix-agent2` installed and running
- `sudo` installed

### Zabbix Server

- Zabbix 6.0 or newer (template is in 6.0 XML format)

---

## Part 1 — Build the Go Binaries

Clone the repository and build:

```bash
git clone https://github.com/jniltinho/zabbix-postfix
cd zabbix-postfix

# Build all three binaries into dist/ inside each module
make build

# Verify
ls -lh pygtail/dist/pygtail pflogsumm/dist/pflogsumm check_mailq/dist/check_mailq
```

Expected output (sizes after UPX compression):

```
-rwxr-xr-x  pygtail/dist/pygtail      ~1.0 MB
-rwxr-xr-x  pflogsumm/dist/pflogsumm  ~1.1 MB
-rwxr-xr-x  check_mailq/dist/check_mailq ~1.1 MB
```

Run unit tests to verify correctness:

```bash
make test
```

---

## Part 2 — Install on the Mail Server (Zabbix Agent Host)

### 2.1 Copy binaries to the mail server

```bash
# From your build machine:
scp pygtail/dist/pygtail     mx01:/tmp/
scp pflogsumm/dist/pflogsumm mx01:/tmp/
scp check_mailq/dist/check_mailq mx01:/tmp/

ssh mx01 "sudo install -m 0755 /tmp/pygtail     /usr/local/bin/pygtail && \
          sudo install -m 0755 /tmp/pflogsumm   /usr/local/bin/pflogsumm && \
          sudo install -m 0755 /tmp/check_mailq /usr/local/bin/check_mailq"
```

Or install directly if you have the repo on the mail server:

```bash
sudo make install
```

Verify:

```bash
/usr/local/bin/pygtail --version
/usr/local/bin/pflogsumm --version
/usr/local/bin/check_mailq --version
```

### 2.2 Run the installer

The installer detects whether `zabbix-agent` or `zabbix-agent2` is in use and installs to the correct conf directory.

```bash
# Run from the repo root on the mail server (as root)
sudo bash install_postfix_template_zabbix_passive.sh
```

The installer:
1. Checks Go binaries are present in `/usr/local/bin/`
2. Copies `zabbix_postfix_passive.sh` to `/usr/local/sbin/`
3. Detects agent conf dir (`zabbix_agent2.d` → `zabbix_agentd.conf.d` → `zabbix_agentd.d`)
4. Copies `zabbix_postfix_passive.conf` to the detected dir
5. Adds the sudoers entry
6. Restarts the Zabbix agent

### 2.3 Manual installation (alternative)

If you prefer manual steps:

```bash
# 1. Install passive script
sudo install -m 0755 zabbix_postfix_passive.sh /usr/local/sbin/

# 2. Install Zabbix agent UserParameter conf
# For zabbix-agent2:
sudo install -m 0644 zabbix_postfix_passive.conf /etc/zabbix/zabbix_agent2.d/
# For zabbix-agent (classic):
sudo install -m 0644 zabbix_postfix_passive.conf /etc/zabbix/zabbix_agentd.conf.d/

# 3. Add sudoers entry
echo 'zabbix ALL=(ALL) NOPASSWD: /usr/local/sbin/zabbix_postfix_passive.sh' \
  | sudo EDITOR='tee -a' visudo

# 4. Restart agent
sudo systemctl restart zabbix-agent2   # or zabbix-agent
```

---

## Part 3 — Configure Zabbix Server

### 3.1 Import the template

1. Open Zabbix web interface
2. Go to **Configuration** → **Templates** → **Import** (top-right button)
3. Upload `template_postfix_passive.xml`
4. Click **Import**

The template creates:
- **14 items** (SMTP check, mail queue, 11 mail metrics, data update trigger)
- **4 graphs** (Message Flow, Bytes Transferred, Mail Queue, Rejected/Bounced)
- **4 triggers** (SMTP down, queue high, deferred high, rejected high)

### 3.2 Link the template to the host

1. Go to **Configuration** → **Hosts**
2. Click on your mail server host
3. Open the **Templates** tab
4. In **Link new templates**, search for `Template App Postfix by Zabbix agent`
5. Click **Add** → **Update**

### 3.3 Verify data collection

After a few minutes, go to **Monitoring** → **Latest data**, filter by the host, and check for `postfix.*` items collecting data.

---

## Part 4 — Verify on the Agent Host

Use `zabbix_get` directly on the mail server to simulate Zabbix server polling:

```bash
# Run the update pipeline (reads new log lines, saves to statsfile)
zabbix_get -s 127.0.0.1 -k 'postfix.update_data'
# Expected: OK: statistics updated

# Read individual metrics
zabbix_get -s 127.0.0.1 -k 'postfix[received]'
# Expected: integer (e.g. 142178)

zabbix_get -s 127.0.0.1 -k 'postfix[delivered]'
zabbix_get -s 127.0.0.1 -k 'postfix[rejected]'
zabbix_get -s 127.0.0.1 -k 'postfix[deferred]'
zabbix_get -s 127.0.0.1 -k 'postfix[bytes_received]'

# Live queue depth
zabbix_get -s 127.0.0.1 -k 'postfix.pfmailq'
# Expected: integer (e.g. 11)
```

You can also run the script directly:

```bash
# Update mode
sudo /usr/local/sbin/zabbix_postfix_passive.sh
# → OK: statistics updated

# Read mode
sudo /usr/local/sbin/zabbix_postfix_passive.sh received
# → 142178

# Queue depth
/usr/local/bin/check_mailq
# → 11
```

---

## Part 5 — Files Reference

### Files deployed to the mail server

| File | Destination | Purpose |
|------|-------------|---------|
| `pygtail` binary | `/usr/local/bin/pygtail` | Incremental log reader |
| `pflogsumm` binary | `/usr/local/bin/pflogsumm` | Postfix log parser |
| `check_mailq` binary | `/usr/local/bin/check_mailq` | Queue depth counter |
| `zabbix_postfix_passive.sh` | `/usr/local/sbin/` | Orchestration script |
| `zabbix_postfix_passive.conf` | `/etc/zabbix/zabbix_agent2.d/` | Zabbix UserParameters |
| sudoers entry | `/etc/sudoers` | Allow zabbix to run the script |

### Runtime files (created automatically)

| File | Purpose |
|------|---------|
| `/tmp/zabbix-postfix-passive-offset.dat` | pygtail offset (log position) |
| `/tmp/zabbix-postfix-passive-statsfile.dat` | Accumulated metrics (`key;value`) |

### Files imported to Zabbix Server

| File | Purpose |
|------|---------|
| `template_postfix_passive.xml` | Zabbix 6.0 template |

---

## Part 6 — Zabbix UserParameters

The conf file defines three UserParameters:

```
# Queue depth — calls check_mailq directly, no sudo needed
UserParameter=postfix.pfmailq,/usr/local/bin/check_mailq

# Read a single metric from the stats file
UserParameter=postfix[*],sudo /usr/local/sbin/zabbix_postfix_passive.sh $1

# Trigger the update pipeline (pygtail | pflogsumm → statsfile)
UserParameter=postfix.update_data,sudo /usr/local/sbin/zabbix_postfix_passive.sh
```

### Available metrics for `postfix[*]`

| Key | Description |
|-----|-------------|
| `postfix[received]` | Messages accepted by the MTA |
| `postfix[delivered]` | Messages successfully delivered |
| `postfix[forwarded]` | Messages forwarded |
| `postfix[deferred]` | Unique messages with at least one deferral |
| `postfix[bounced]` | Messages bounced (`status=bounced`) |
| `postfix[rejected]` | Messages rejected (NOQUEUE + milter-reject) |
| `postfix[reject_warnings]` | Reject warnings |
| `postfix[held]` | Messages held |
| `postfix[discarded]` | Messages discarded |
| `postfix[bytes_received]` | Total bytes received (integer, no suffixes) |
| `postfix[bytes_delivered]` | Total bytes delivered (integer, no suffixes) |

---

## Part 7 — How the Update Cycle Works

```
Every 1 minute (postfix.update_data):
    pygtail -o /tmp/...offset.dat /var/log/mail.log
        │  reads only NEW lines since last run
        │  tracks log rotation automatically
        ▼
    pflogsumm (stdin)
        │  outputs key=value metrics:
        │    received=42
        │    delivered=38
        │    bytes_received=1048576
        │    ...
        ▼
    zabbix_postfix_passive.sh
        │  adds new values to existing stats file:
        │    received;184     (was 142, +42)
        │    delivered;312    (was 274, +38)
        │    ...

Every 3 minutes (postfix[received], postfix[delivered], ...):
    reads single value from statsfile → integer → Zabbix
```

Metrics are **cumulative** — they accumulate since the last zabbix-agent restart or manual statsfile reset. Graphs show the rate of change using Zabbix's built-in derivative functions.

---

## Part 8 — Environment Overrides (for testing)

Override binary paths without editing the script:

```bash
# Test with a different log file
sudo MAILLOG=/var/log/mail.log.1 /usr/local/sbin/zabbix_postfix_passive.sh

# Test with custom binaries
ZABBIX_POSTFIX_PFLOGSUMM=/tmp/pflogsumm_new \
ZABBIX_POSTFIX_PYGTAIL=/tmp/pygtail_new \
  sudo -E /usr/local/sbin/zabbix_postfix_passive.sh
```

---

## Part 9 — Troubleshooting

### No data in Zabbix

```bash
# Check the agent log
sudo journalctl -u zabbix-agent2 -n 50

# Check UserParameters are loaded
sudo zabbix_agent2 -t postfix.update_data
sudo zabbix_agent2 -t 'postfix[received]'
```

### "ERROR: ... not found"

Go binaries are not installed or not executable:

```bash
ls -lh /usr/local/bin/pygtail /usr/local/bin/pflogsumm /usr/local/bin/check_mailq
# If missing: sudo make install (from repo root)
```

### "ERROR: ... not readable"

The mail log path is not readable by zabbix:

```bash
ls -l /var/log/mail.log
# Typically: -rw-r----- root adm
# Add zabbix to adm group:
sudo usermod -aG adm zabbix
sudo systemctl restart zabbix-agent2
```

### statsfile shows wrong or zero values

Reset the offset to reprocess the full log:

```bash
sudo rm -f /tmp/zabbix-postfix-passive-offset.dat /tmp/zabbix-postfix-passive-statsfile.dat
# Next update run will reprocess mail.log from the start
```

### validate with Docker (integration test)

```bash
# Build binaries first
make build

# Run full validation suite in Ubuntu 24.04 (19 tests)
docker build -f Dockerfile.test-passive -t zabbix-postfix-test .
docker run --rm zabbix-postfix-test
# Expected: Results: 19 passed, 0 failed
```

---

## Part 10 — Rollback

If you need to revert to a previous version:

```bash
# Remove installed files
sudo rm -f /usr/local/bin/pygtail /usr/local/bin/pflogsumm /usr/local/bin/check_mailq
sudo rm -f /usr/local/sbin/zabbix_postfix_passive.sh
sudo rm -f /etc/zabbix/zabbix_agent2.d/zabbix_postfix_passive.conf

# Remove sudoers entry
sudo sed -i '/zabbix_postfix_passive/d' /etc/sudoers

# Restart agent
sudo systemctl restart zabbix-agent2

# Stats and offset files remain intact — compatible with any future reinstall
```

---

## Part 11 — Migration from Python/Perl

If you were previously using `pygtail.py` and Perl `pflogsumm`:

- **Offset file** (`/tmp/zabbix-postfix-passive-offset.dat`) — same format, no reset needed
- **Stats file** (`/tmp/zabbix-postfix-passive-statsfile.dat`) — same `key;value` format, no reset needed
- **Template** — same item keys, no template re-import needed
- **Packages to remove** — `apt remove pflogsumm` (optional, no longer required)
