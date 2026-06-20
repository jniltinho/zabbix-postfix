# zabbix-postfix

**Monitor Postfix in Zabbix — without Python or Perl on your mail server.**

Drop-in replacement for the classic `pygtail.py` + Perl `pflogsumm` stack. Three small Go binaries, a ready-made Zabbix 6.0 template, and an installer that wires everything up on the agent host.

[![Release](https://img.shields.io/github/v/release/jniltinho/zabbix-postfix)](https://github.com/jniltinho/zabbix-postfix/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> **New here?** Start with the **[HOWTO](./docs/HOWTO.md)** — step-by-step install, template import, and troubleshooting.

---

## Why use this?

If you run Postfix and Zabbix, you probably want message flow metrics — received, delivered, bounced, rejected, queue depth — without maintaining a fragile toolchain on every mail server.

| Before | With zabbix-postfix |
|--------|---------------------|
| `apt install pflogsumm` + Python `pygtail.py` | Three static Go binaries (~1 MB each, UPX-compressed) |
| Perl + Python runtime on the agent host | No interpreter dependencies |
| Shell pipelines parsing `mailq` output | `check_mailq` returns a single integer |
| Custom scripts to glue it all together | Installer + Zabbix template included |

**Drop-in compatible** — same offset file, stats file format, and Zabbix item keys. Migrate from the Python/Perl setup without resetting counters or re-importing a different template.

---

## What you get

Import `template_postfix_passive.xml` and monitor:

| Metric | What it tells you |
|--------|-------------------|
| Received / Delivered | Message throughput |
| Deferred / Bounced / Rejected | Delivery problems and policy blocks |
| Bytes received / delivered | Traffic volume |
| Mail queue depth | Backlog building up right now |
| SMTP service state | Is Postfix accepting connections? |

The template ships with **14 items**, **4 graphs**, and **4 triggers** (high queue, deferred spike, reject spike, SMTP down).

![How it works](./docs/screenshots/how-it-works.jpg)

*See also:*
* [Zabbix-Postfix Integration Flow Diagram](./docs/screenshots/postfix_zabbix_flow.jpg)
* [Postfix Mail Server Delivery Flow Diagram](./docs/screenshots/postfix_delivery_flow.jpg)
* [Postfix Mail Server Statistics Overview (For Beginners)](./docs/screenshots/postfix_stats_infographic.jpg)
* [Postfix Zabbix Agent Metrics Reference](./docs/screenshots/postfix_metrics_reference.jpg)

Every 1–3 minutes the Zabbix server polls the agent. `postfix.update_data` tails the mail log incrementally (`pygtail → pflogsumm --zabbix`) and accumulates counters. `postfix[*]` reads a single metric from the stats file. `postfix.pfmailq` queries the live queue via `check_mailq`.

---

## Quick Start

### Option A — Download pre-built binaries (fastest)

Grab the latest release and install on your mail server:

```bash
VERSION=0.0.1   # or check https://github.com/jniltinho/zabbix-postfix/releases
BASE="https://github.com/jniltinho/zabbix-postfix/releases/download/v${VERSION}"

for bin in pygtail pflogsumm check_mailq; do
  curl -fsSL "${BASE}/${bin}_${VERSION}_linux_amd64.tar.gz" | sudo tar -xz -C /usr/local/bin/
done
```

Then clone the repo on the mail server (for the installer and template) and run:

```bash
git clone https://github.com/jniltinho/zabbix-postfix
cd zabbix-postfix
sudo bash install_postfix_template_zabbix_passive.sh
```

Import `template_postfix_passive.xml` in Zabbix Server and link it to your mail host. Full details in **[HOWTO.md](./docs/HOWTO.md)**.

### Option B — Build from source

```bash
git clone https://github.com/jniltinho/zabbix-postfix
cd zabbix-postfix

make build          # binaries in */dist/, compressed with UPX
make test           # unit tests across all modules
sudo make install   # installs to /usr/local/bin/
sudo bash install_postfix_template_zabbix_passive.sh
```

**Requirements:** Go ≥ 1.21, `make`, `upx`. Agent host needs Postfix, Zabbix Agent/Agent2, and `sudo`.

---

## Components

| Module | Replaces | Role |
|--------|----------|------|
| [`pygtail`](./pygtail/) | `pygtail.py` | Incremental log reader with rotation and `.gz` support |
| [`pflogsumm`](./pflogsumm/) | Perl `pflogsumm` | Log parser — human report by default, `--zabbix` for metrics |
| [`check_mailq`](./check_mailq/) | `mailq \| grep` pipeline | Live queue depth as a raw integer |

**Deploy on the agent host:**

| File | Destination |
|------|-------------|
| `zabbix_postfix_passive.sh` | `/usr/local/sbin/` |
| `zabbix_postfix_passive.conf` | `/etc/zabbix/zabbix_agent2.d/` |
| `zabbix_postfix_passive` | `/etc/sudoers.d/` |

**Import on Zabbix Server:**

| File | Purpose |
|------|---------|
| `template_postfix_passive.xml` | Zabbix 6.0 template |

---

## Module highlights

- **[pygtail](./pygtail/README.md)** — offset file format identical to `pygtail.py` v0.11.1
- **[pflogsumm](./pflogsumm/README.md)** — output matches Perl pflogsumm; `-d today|yesterday`, `--mailq`, and all compat flags accepted
- **[check_mailq](./check_mailq/README.md)** — same count as `mailq | grep -v "Mail queue is empty" | grep -c '^[0-9A-Z]'`

---

## Validate before you deploy

Run the integration suite in Docker — no mail server required:

```bash
make build
docker build -f Dockerfile.test-passive -t zabbix-postfix-test .
docker run --rm zabbix-postfix-test
# Expected: Results: 19 passed, 0 failed
```

---

## Roadmap

| Step | Status |
|------|--------|
| Go binaries | ✓ done — golden test matches Perl pflogsumm exactly |
| Zabbix wiring | ✓ done — validated via Docker (19/19) |
| Native Zabbix plugin | planned — `check_postfix` importing `pflogsumm/pkg/parser` |

---

## Documentation

| Doc | Contents |
|-----|----------|
| **[HOWTO.md](./docs/HOWTO.md)** | Full install guide, template import, troubleshooting, migration from Python/Perl |
| **[Motivation](./docs/motivation.md)** | Why choose Go? Benefits over Perl/Python runtimes and impact on Zabbix metrics |
| **[pygtail](./pygtail/README.md)** | Log tailing, offset files, rotation |
| **[pflogsumm](./pflogsumm/README.md)** | Output modes, flags, examples |
| **[check_mailq](./check_mailq/README.md)** | Queue counting |

---

## License

[MIT](LICENSE) — Copyright (c) 2026 Nilton Oliveira