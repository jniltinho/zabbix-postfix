# zabbix-postfix

**Monitor Postfix email traffic in Zabbix — without Python or Perl.**

[![Release](https://img.shields.io/github/v/release/jniltinho/zabbix-postfix)](https://github.com/jniltinho/zabbix-postfix/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

![How it works](./docs/screenshots/how-it-works.jpg)

---

## What it does

Installs three small Go binaries on your mail server and a ready-made template on
Zabbix. Together they collect Postfix statistics every 1–3 minutes and display them
as graphs and alerts — no configuration needed beyond `dpkg -i` or `rpm -i`.

| Metric | What it tells you |
|--------|-------------------|
| Received / Delivered | Message throughput |
| Deferred / Bounced / Rejected | Delivery problems |
| Bytes received / delivered | Traffic volume |
| Mail queue depth | Backlog right now |
| SMTP service state | Is Postfix accepting connections? |

Template ships with **14 items**, **4 graphs**, and **4 triggers**.

---

## Quick Start

### 1. Download the package for your distro

Go to the **[Releases page](https://github.com/jniltinho/zabbix-postfix/releases)**
and download the file for your server:

| Distro | File to download |
|--------|-----------------|
| Debian / Ubuntu | `zabbix-postfix_<version>_amd64.deb` |
| RHEL / CentOS / Rocky / AlmaLinux | `zabbix-postfix-<version>-1.x86_64.rpm` |
| Any Linux | `zabbix-postfix_<version>_pkg_linux_amd64.tar.gz` |

### 2. Install on the mail server

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
mkdir -p /tmp/zabbix-postfix
tar -xzf /tmp/zabbix-postfix_*_pkg_linux_amd64.tar.gz -C /tmp/zabbix-postfix
sudo bash /tmp/zabbix-postfix/usr/share/zabbix-postfix/install.sh
```

The installer automatically detects your Zabbix agent, installs the config, adds
the sudoers entry, and restarts the agent.

### 3. Import the template in Zabbix

1. **Configuration → Templates → Import**
2. Upload `template_postfix_passive.xml`
   (located at `/usr/share/zabbix-postfix/` after install)
3. Link the template to your mail server host

After a few minutes, **Monitoring → Latest data** will show `postfix.*` metrics.

---

## Verify it works

```bash
zabbix_get -s 127.0.0.1 -k 'postfix.update_data'  # OK: statistics updated
zabbix_get -s 127.0.0.1 -k 'postfix[received]'    # integer, e.g. 142178
zabbix_get -s 127.0.0.1 -k 'postfix.pfmailq'      # integer, e.g. 11
```

---

## No toolchain required on the mail server

The three binaries are statically compiled and UPX-compressed (~1 MB each).
No Go, Python, Perl, or any runtime dependency needed on the agent host.

| What you get | Details |
|--------------|---------|
| `pygtail` | Reads new log lines since last run (survives log rotation) |
| `pflogsumm` | Parses Postfix logs and outputs counters |
| `check_mailq` | Returns live queue depth as a single integer |
| `zabbix_postfix_passive.sh` | Orchestrates the three binaries for Zabbix |
| `template_postfix_passive.xml` | Zabbix 7.0 template |

---

## Don't have releases? Build with Docker

No Go, UPX, or fpm needed — just Docker:

```bash
git clone https://github.com/jniltinho/zabbix-postfix
cd zabbix-postfix
bash scripts/build-packages-docker.sh
# packages written to dist/
```

---

## Documentation

| Doc | Contents |
|-----|----------|
| **[HOWTO.md](./docs/HOWTO.md)** | Step-by-step install guide |
| **[DEVELOPMENT.md](./docs/DEVELOPMENT.md)** | Build, test, release, advanced config |
| **[Motivation](./docs/motivation.md)** | Why Go instead of Perl/Python |

---

## Migrating from Python/Perl?

Drop-in compatible — same offset file, stats file format, and Zabbix item keys.
No need to reset counters or re-import a different template.

---

## License

[MIT](LICENSE) — Copyright (c) 2026 Nilton Oliveira
