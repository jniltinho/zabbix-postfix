# HOWTO — zabbix-postfix

Monitor Postfix with Zabbix in three steps.

---

## What this does

Installs one Go binary on your mail server and a template on the Zabbix server.
Together they collect Postfix statistics (received, delivered, rejected, queue depth,
etc.) and display them as graphs and alerts.

No Python, no Perl, no system packages needed on the mail server.

---

## Prerequisites

**Mail server:**
- Postfix writing to `/var/log/mail.log` (Debian/Ubuntu) or `/var/log/maillog` (RHEL)
- `zabbix-agent` or `zabbix-agent2` installed and running
- `sudo` installed

**Zabbix server:** version 6.0 or newer

---

## Step 1 — Get the package

**Option A — Download from GitHub releases (recommended)**

Go to the [Releases page](https://github.com/jniltinho/zabbix-postfix/releases) and
download the file for your distro:

| File | Distro |
|------|--------|
| `zabbix-postfix_<version>_amd64.deb` | Debian, Ubuntu |
| `zabbix-postfix-<version>-1.x86_64.rpm` | RHEL, CentOS, Rocky, AlmaLinux |
| `zabbix-postfix_<version>_pkg_linux_amd64.tar.gz` | Any Linux |

**Option B — Build with Docker** (no Go, UPX, or fpm needed)

```bash
git clone https://github.com/jniltinho/zabbix-postfix
cd zabbix-postfix
bash scripts/build-packages-docker.sh
# packages written to dist/
```

---

## Step 2 — Install on the mail server

**.deb**

```bash
scp zabbix-postfix_*.deb mailserver:/tmp/
sudo dpkg -i /tmp/zabbix-postfix_*.deb
```

**.rpm**

```bash
scp zabbix-postfix-*.rpm mailserver:/tmp/
sudo rpm -i /tmp/zabbix-postfix-*.rpm
```

**.tar.gz**

```bash
scp zabbix-postfix_*_pkg_linux_amd64.tar.gz mailserver:/tmp/
# On the mail server:
mkdir -p /tmp/zabbix-postfix
tar -xzf /tmp/zabbix-postfix_*_pkg_linux_amd64.tar.gz -C /tmp/zabbix-postfix
sudo bash /tmp/zabbix-postfix/usr/share/zabbix-postfix/install.sh
```

All three methods automatically detect your Zabbix agent, install the config,
add the sudoers entry, and restart the agent.

> **Custom paths** (e.g. `/opt/zabbix_bin`): use the `.tar.gz` installer with
> `--bin-dir /opt/zabbix_bin --script-dir /opt/zabbix_bin`. See [DEVELOPMENT.md](DEVELOPMENT.md#custom-binary-paths).

### Verify

```bash
zabbix_get -s 127.0.0.1 -k 'postfix.update_data'  # OK: statistics updated
zabbix_get -s 127.0.0.1 -k 'postfix[received]'    # integer, e.g. 142178
zabbix_get -s 127.0.0.1 -k 'postfix.pfmailq'      # integer, e.g. 11
```

---

## Step 3 — Import the template on Zabbix

1. **Configuration → Templates → Import**
2. Upload `template_postfix_passive.xml`
   - `.deb`/`.rpm`: `/usr/share/zabbix-postfix/template_postfix_passive.xml`
   - `.tar.gz`: `/tmp/zabbix-postfix/usr/share/zabbix-postfix/template_postfix_passive.xml`
3. **Configuration → Hosts** → your mail server → **Templates** tab
4. Search `Template App Postfix by Zabbix agent` → **Add → Update**

After a few minutes check **Monitoring → Latest data** for `postfix.*` items.

---

## Troubleshooting

**No data in Zabbix**
```bash
sudo journalctl -u zabbix-agent2 -n 50
sudo zabbix_agent2 -t postfix.update_data
```

**Binary not found**
```bash
ls -lh /opt/zabbix_postfix/pflogsumm
```

**Mail log not readable**
```bash
sudo usermod -aG adm zabbix && sudo systemctl restart zabbix-agent2
```

**Zero or stale metrics**
```bash
sudo /opt/zabbix_postfix/zabbix_postfix_passive.sh --reset
zabbix_get -s 127.0.0.1 -k 'postfix.update_data'
```

For full details — architecture, build instructions, testing, release process,
advanced configuration — see [DEVELOPMENT.md](DEVELOPMENT.md).
