# HOWTO — zabbix-postfix

Step-by-step guide to monitor Postfix with Zabbix using the Go binaries from this repository.

---

## What this does

zabbix-postfix installs three small programs on your mail server and a template on
the Zabbix server. Together they collect Postfix statistics (messages received,
delivered, rejected, queue depth, etc.) and display them as graphs and alerts in
Zabbix.

```
Zabbix Server                        Mail server (Zabbix agent)
─────────────                        ──────────────────────────
template_postfix_passive.xml         zabbix_postfix_passive.conf
  14 items, 4 graphs, 4 triggers  ◀────  3 UserParameters
                                             │
                         postfix.update_data ──▶ zabbix_postfix_passive.sh
                                             │       ├─ pygtail   (reads new log lines)
                                             │       └─ pflogsumm (parses and counts)
                         postfix[received]   ──▶ reads saved counter → integer
                         postfix[delivered]  ──▶ reads saved counter → integer
                         postfix.pfmailq     ──▶ check_mailq → live queue depth
```

No Python, no Perl, no system packages needed on the mail server — only the three
Go binaries from this repo.

---

## Prerequisites

Before you start, make sure you have:

**On the build machine** (your laptop or a CI server — where you will compile):

- Go ≥ 1.26.4 **and** UPX (`apt install upx-ucl`) — **OR** just Docker (no Go needed)
- `make`

**On the mail server** (where Postfix and the Zabbix agent run):

- Postfix writing to `/var/log/mail.log` (Debian/Ubuntu) or `/var/log/maillog` (RHEL/CentOS)
- `zabbix-agent` or `zabbix-agent2` installed and running
- `sudo` installed

**On the Zabbix server**:

- Zabbix 6.0 or newer

---

## Step 1 — Build the Go binaries

Clone the repository on your build machine:

```bash
git clone https://github.com/jniltinho/zabbix-postfix
cd zabbix-postfix
```

Then choose one of the two build methods:

### Option A — Local build (Go + UPX installed)

```bash
make build
```

### Option B — Docker build (no Go or UPX needed)

```bash
bash scripts/build-binaries-docker.sh
```

Either way, the result is three compressed binaries:

```
pygtail/dist/pygtail        ~1.0 MB
pflogsumm/dist/pflogsumm    ~1.1 MB
check_mailq/dist/check_mailq ~1.1 MB
```

Run unit tests to confirm correctness:

```bash
make test
```

---

## Step 2 — Create an install package

Bundle the binaries, scripts, and Zabbix template into one folder that you can
copy to any server — no git clone needed on the mail server.

```bash
# Use the binaries you just built
bash scripts/make-install-package.sh

# Build with Docker and package in one step
bash scripts/make-install-package.sh --docker

# Also create a .tar.gz for easy transfer
bash scripts/make-install-package.sh --docker --archive
```

The package is written to `dist/zabbix-postfix-install/`:

```
dist/zabbix-postfix-install/
├── INSTALL.txt
├── bin/
│   ├── pygtail
│   ├── pflogsumm
│   └── check_mailq
├── install_postfix_template_zabbix_passive.sh   ← run this on the mail server
├── zabbix_postfix_passive.sh
├── zabbix_postfix_passive.conf
├── template_postfix_passive.xml                 ← import this on the Zabbix server
└── scripts/
    └── configure_paths.sh
```

---

## Step 3 — Install on the mail server

### 3.1 Copy the package to the mail server

```bash
# On your build machine
scp dist/zabbix-postfix-install.tar.gz mailserver:/tmp/

# On the mail server
cd /tmp
tar -xzf zabbix-postfix-install.tar.gz
cd zabbix-postfix-install
```

### 3.2 Copy the binaries

```bash
sudo install -m 0755 bin/pygtail bin/pflogsumm bin/check_mailq /usr/local/bin/
```

Verify:

```bash
/usr/local/bin/pygtail --version
/usr/local/bin/pflogsumm --version
/usr/local/bin/check_mailq --version
```

### 3.3 Run the installer

```bash
sudo bash install_postfix_template_zabbix_passive.sh
```

The installer does the following automatically:

1. Confirms the three Go binaries are in `/usr/local/bin/`
2. Copies `zabbix_postfix_passive.sh` to `/usr/local/sbin/`
3. Detects your Zabbix agent config directory (`zabbix_agent2.d`, `zabbix_agentd.conf.d`, or `zabbix_agentd.d`)
4. Copies `zabbix_postfix_passive.conf` to that directory
5. Adds a sudoers entry so the `zabbix` user can read the mail log
6. Restarts the Zabbix agent

If you prefer to do it manually, see [Manual installation](#manual-installation) below.

### 3.4 Verify the agent is working

Use `zabbix_get` to simulate what the Zabbix server will poll:

```bash
# Trigger the update pipeline (reads new log lines and saves counters)
zabbix_get -s 127.0.0.1 -k 'postfix.update_data'
# Expected: OK: statistics updated

# Read a metric
zabbix_get -s 127.0.0.1 -k 'postfix[received]'
# Expected: an integer, e.g. 142178

# Live queue depth
zabbix_get -s 127.0.0.1 -k 'postfix.pfmailq'
# Expected: an integer, e.g. 11
```

---

## Step 4 — Import the template on the Zabbix server

1. Open the Zabbix web interface
2. Go to **Configuration → Templates → Import** (button at the top right)
3. Upload `template_postfix_passive.xml`
4. Click **Import**

The template adds:

| What | Details |
|------|---------|
| **14 items** | SMTP check, queue depth, 11 mail counters, data update trigger |
| **4 graphs** | Message Flow, Bytes Transferred, Mail Queue, Rejected/Bounced |
| **4 triggers** | SMTP down, queue high, deferred high, rejected high |

### Link the template to your host

1. Go to **Configuration → Hosts**
2. Click on your mail server
3. Open the **Templates** tab
4. Search for `Template App Postfix by Zabbix agent`
5. Click **Add → Update**

After a few minutes, go to **Monitoring → Latest data**, filter by your host, and
look for `postfix.*` items with data.

---

## How it works in detail

### The update cycle

Zabbix polls two types of items on different schedules:

```
Every 1 minute — postfix.update_data
    pygtail reads only NEW lines from mail.log since last run
        │  (tracks position with an offset file — survives log rotation)
        ▼
    pflogsumm --zabbix parses them and outputs key=value:
        received=42
        delivered=38
        bytes_received=1048576
        ...
        ▼
    zabbix_postfix_passive.sh adds the delta to the stats file:
        received;184   (was 142 + 42)
        delivered;312  (was 274 + 38)
        ...

Every 3 minutes — postfix[received], postfix[delivered], ...
    zabbix_postfix_passive.sh reads one line from the stats file → integer → Zabbix
```

Metrics are **cumulative** — they grow until the stats file is deleted or the
agent is reinstalled. Zabbix graphs the rate of change, not the raw total.

### Available metrics

| Zabbix key | Description |
|------------|-------------|
| `postfix[received]` | Messages accepted by Postfix |
| `postfix[delivered]` | Messages delivered to final destination |
| `postfix[forwarded]` | Messages forwarded via aliases |
| `postfix[deferred]` | Messages with at least one temporary failure |
| `postfix[bounced]` | Messages permanently rejected by destination |
| `postfix[rejected]` | Messages rejected at SMTP level (before queuing) |
| `postfix[reject_warnings]` | Reject warnings (warn_if_reject) |
| `postfix[held]` | Messages held by administrator |
| `postfix[discarded]` | Messages silently discarded |
| `postfix[bytes_received]` | Total bytes received |
| `postfix[bytes_delivered]` | Total bytes delivered |
| `postfix.pfmailq` | Current mail queue depth (live) |
| `postfix.update_data` | Triggers the update pipeline |

### UserParameters

The file `/etc/zabbix/zabbix_agent2.d/zabbix_postfix_passive.conf` registers
these commands with the Zabbix agent:

```
UserParameter=postfix.pfmailq,/usr/local/bin/check_mailq
UserParameter=postfix[*],sudo /usr/local/sbin/zabbix_postfix_passive.sh $1
UserParameter=postfix.update_data,sudo /usr/local/sbin/zabbix_postfix_passive.sh
```

### Runtime files

Two temporary files are created automatically and must not be deleted while the
agent is running:

| File | Purpose |
|------|---------|
| `/tmp/zabbix-postfix-passive-offset.dat` | Where `pygtail` stopped reading last time |
| `/tmp/zabbix-postfix-passive-statsfile.dat` | Accumulated counters (`key;value` per line) |

---

## Troubleshooting

### No data appears in Zabbix

Check the agent log:

```bash
sudo journalctl -u zabbix-agent2 -n 50
```

Test the UserParameters directly on the mail server:

```bash
sudo zabbix_agent2 -t postfix.update_data
sudo zabbix_agent2 -t 'postfix[received]'
```

### "ERROR: ... not found or not executable"

The Go binaries are missing:

```bash
ls -lh /usr/local/bin/pygtail /usr/local/bin/pflogsumm /usr/local/bin/check_mailq
# If missing, re-run: sudo install -m 0755 bin/* /usr/local/bin/
```

### "ERROR: ... not readable" (mail log)

The `zabbix` user cannot read the mail log:

```bash
ls -l /var/log/mail.log
# Typically: -rw-r----- root adm
# Fix: add zabbix to the adm group
sudo usermod -aG adm zabbix
sudo systemctl restart zabbix-agent2
```

### Metrics show zero or stale values

Reset the offset file to reprocess the full log from the beginning:

```bash
sudo rm -f /tmp/zabbix-postfix-passive-offset.dat \
           /tmp/zabbix-postfix-passive-statsfile.dat
zabbix_get -s 127.0.0.1 -k 'postfix.update_data'
```

### Run the integration test suite

```bash
make build
docker build -f Dockerfile.test-passive -t zabbix-postfix-test .
docker run --rm zabbix-postfix-test
# Expected: Results: 19 passed, 0 failed
```

---

## Advanced topics

### Manual installation

If you prefer not to use the installer script:

```bash
# 1. Install the passive script
sudo install -m 0755 zabbix_postfix_passive.sh /usr/local/sbin/

# 2. Install the Zabbix agent UserParameter conf
# For zabbix-agent2:
sudo install -m 0644 zabbix_postfix_passive.conf /etc/zabbix/zabbix_agent2.d/
# For zabbix-agent (classic):
sudo install -m 0644 zabbix_postfix_passive.conf /etc/zabbix/zabbix_agentd.conf.d/

# 3. Add the sudoers entry
echo 'zabbix ALL=(ALL) NOPASSWD: /usr/local/sbin/zabbix_postfix_passive.sh' \
  | sudo EDITOR='tee -a' visudo

# 4. Restart the agent
sudo systemctl restart zabbix-agent2   # or zabbix-agent
```

### Custom binary paths

By default the binaries go to `/usr/local/bin/`. If your server policy requires a
different directory (e.g. `/opt/zabbix_bin/`):

```bash
# 1. Copy the binaries
sudo mkdir -p /opt/zabbix_bin
sudo install -m 0755 bin/pygtail bin/pflogsumm bin/check_mailq /opt/zabbix_bin/

# 2. Update the installed scripts to point to the new directory
sudo bash scripts/configure_paths.sh --bin-dir /opt/zabbix_bin
```

The script accepts two options:

| Option | Default | Description |
|--------|---------|-------------|
| `--bin-dir DIR` | `/usr/local/bin` | Directory of `pygtail`, `pflogsumm`, `check_mailq` |
| `--script-dir DIR` | `/usr/local/sbin` | Directory of `zabbix_postfix_passive.sh` |

```bash
# Both binaries and shell script in the same custom directory
sudo bash scripts/configure_paths.sh \
  --bin-dir /opt/zabbix_bin \
  --script-dir /opt/zabbix_bin
```

To update paths manually instead:

```bash
# Update binary paths in the passive script
sudo sed -i \
  -e 's|PYGTAIL=\${ZABBIX_POSTFIX_PYGTAIL:-[^}]*}|PYGTAIL=${ZABBIX_POSTFIX_PYGTAIL:-/opt/zabbix_bin/pygtail}|' \
  -e 's|PFLOGSUMM=\${ZABBIX_POSTFIX_PFLOGSUMM:-[^}]*}|PFLOGSUMM=${ZABBIX_POSTFIX_PFLOGSUMM:-/opt/zabbix_bin/pflogsumm}|' \
  /usr/local/sbin/zabbix_postfix_passive.sh

# Update check_mailq path in the agent conf
sudo sed -i \
  's|UserParameter=postfix\.pfmailq,.*|UserParameter=postfix.pfmailq,/opt/zabbix_bin/check_mailq|' \
  /etc/zabbix/zabbix_agent2.d/zabbix_postfix_passive.conf

sudo systemctl restart zabbix-agent2
```

### Environment variable overrides (for testing)

Override paths without editing files:

```bash
# Test with a rotated log file
sudo MAILLOG=/var/log/mail.log.1 /usr/local/sbin/zabbix_postfix_passive.sh

# Test with a different binary
ZABBIX_POSTFIX_PFLOGSUMM=/tmp/pflogsumm_new \
ZABBIX_POSTFIX_PYGTAIL=/tmp/pygtail_new \
  sudo -E /usr/local/sbin/zabbix_postfix_passive.sh
```

### Rollback / uninstall

```bash
# Remove installed files
sudo rm -f /usr/local/bin/pygtail \
           /usr/local/bin/pflogsumm \
           /usr/local/bin/check_mailq \
           /usr/local/sbin/zabbix_postfix_passive.sh \
           /etc/zabbix/zabbix_agent2.d/zabbix_postfix_passive.conf

# Remove sudoers entry
sudo sed -i '/zabbix_postfix_passive/d' /etc/sudoers

# Restart agent
sudo systemctl restart zabbix-agent2
```

The offset and stats files in `/tmp/` remain — delete them too if you want a
clean slate.

### Migration from Python/Perl pflogsumm

If you were using the original `pygtail.py` and Perl `pflogsumm`:

- **Offset file** — same format, no reset needed
- **Stats file** — same `key;value` format, no reset needed
- **Zabbix template** — same item keys, no re-import needed
- **pflogsumm** — the Go binary outputs the same `--zabbix` key=value format; pass `--zabbix` explicitly (the passive script already does this)
- **Old packages** — `apt remove pflogsumm` is optional; it is no longer required
