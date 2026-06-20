# Documentation

| Document | Description |
|----------|-------------|
| [HOWTO.md](HOWTO.md) | Step-by-step installation and configuration guide |
| [ZABBIX_API.md](ZABBIX_API.md) | Import template and create scripts via Zabbix JSON-RPC API |
| [motivation.md](motivation.md) | Why Go instead of Perl/Python |
| [pflogsumm-zabbix-output.md](pflogsumm-zabbix-output.md) | pflogsumm `--zabbix` output fields and all flags explained |
| [screenshots/README.md](screenshots/README.md) | Architecture and flow diagrams |

---

## Project files and their role in Zabbix checks

The zabbix-postfix stack splits work between the **Zabbix server** (polls items, stores history, fires triggers) and the **Zabbix agent** on the mail server (runs UserParameters on demand). The files below are the standard pieces that make that work.

```
Zabbix Server                          Mail server (Zabbix agent)
─────────────────                      ────────────────────────────
template_postfix_passive.xml  ──poll──▶  zabbix_postfix_passive.conf
  (items, graphs, triggers)                 │
                                            ├─ postfix.update_data ──▶ zabbix_postfix_passive.sh
                                            ├─ postfix[received]     ──▶     │
                                            ├─ postfix[delivered]    ──▶     │
                                            └─ postfix.pfmailq       ──▶ pflogsumm check-mailq
```

### Zabbix server

| File | Installed where | Role in Zabbix checks |
|------|-----------------|----------------------|
| [`template_postfix_passive.xml`](../template_postfix_passive.xml) | Imported via **Configuration → Templates → Import** | Defines what Zabbix polls and how results are used: **14 items** (SMTP state, queue depth, log-based counters, update trigger), **4 graphs**, and **4 triggers** (SMTP down, high queue, high deferred, high rejected). Without this template, the agent UserParameters exist but nothing collects or graphs the data. |

### Zabbix agent host (mail server)

| File | Installed where | Role in Zabbix checks |
|------|-----------------|----------------------|
| [`zabbix_postfix_passive.conf`](../zabbix_postfix_passive.conf) | `/etc/zabbix/zabbix_agent2.d/` or `zabbix_agentd.conf.d/` | Registers three **UserParameters** so the agent knows how to answer Zabbix item keys: `postfix.update_data` (run the update pipeline), `postfix[*]` (read one metric from the stats file), and `postfix.pfmailq` (live queue depth). This is the bridge between Zabbix item keys and local commands. |
| [`zabbix_postfix_passive.sh`](../zabbix_postfix_passive.sh) | `/opt/zabbix_postfix/` | Core orchestration script. **Without arguments:** parses the last 5 minutes of Postfix log lines (`pflogsumm --zabbix --last 5m`), and caches counters into the stats file — triggered by the `postfix.update_data` item every minute. **With a metric name** (e.g. `received`): reads that value from the stats file — triggered by `postfix[received]`, `postfix[delivered]`, etc. Requires `sudo` because it reads `/var/log/mail.log`. |
| `pflogsumm` (Go binary) | `/opt/zabbix_postfix/pflogsumm` | Postfix log parser and queue checker. Outputs flat `key=value` metrics (`received`, `delivered`, `rejected`, …) when parsing logs, and returns the current mail queue depth as an integer under the `check-mailq` subcommand. |

### Runtime files (created automatically)

| File | Location | Role in Zabbix checks |
|------|----------|----------------------|
| Stats file | `/tmp/zabbix-postfix-passive-statsfile.dat` | Persistent cache file in `key=value` format (e.g. `received=142178`). Written by `postfix.update_data`, read by every `postfix[*]` item. |

### Sudoers entry (not a repo file)

| Entry | Location | Role in Zabbix checks |
|-------|----------|----------------------|
| `zabbix ALL=(ALL) NOPASSWD: /opt/zabbix_postfix/zabbix_postfix_passive.sh` | `/etc/sudoers` (added by the installer) | Allows the `zabbix` agent user to run the passive script as root so it can read the mail log. Without it, `postfix.update_data` and `postfix[*]` return errors. `postfix.pfmailq` does not need sudo. |

### Setup and maintenance scripts (repo only)

| File | Role |
|------|------|
| [`install_postfix_template_zabbix_passive.sh`](../install_postfix_template_zabbix_passive.sh) | Interactive installer on the mail server: verifies Go binaries, deploys the passive script and agent conf, adds the sudoers line, and restarts the agent. Does not import the template — that step is done on the Zabbix server. |
| [`scripts/configure_paths.sh`](../scripts/configure_paths.sh) | Reconfigures binary and script paths in `zabbix_postfix_passive.conf` and `zabbix_postfix_passive.sh`. Default install path is `/opt/zabbix_postfix`; use `--bin-dir` and `--script-dir` to override. |
| [`validate-passive.sh`](../validate-passive.sh) | Integration test script (CI / Docker). Confirms binaries, the update pipeline, stats file format, and read mode behave correctly before deployment. |
| [`Makefile`](../Makefile) | Builds, tests, and installs the Go binary (`make build`, `make test`, `make install`). |
| [`docs/Dockerfile`](Dockerfile) | Docker image (`golang:1.26.4-bookworm`) to compile binaries without Go 1.26.4 or UPX on the host. Used by `scripts/build-binaries-docker.sh`. |
| [`scripts/build-binaries-docker.sh`](../scripts/build-binaries-docker.sh) | Runs the Docker build and copies compressed binaries into `*/dist/`. |
| [`scripts/make-install-package.sh`](../scripts/make-install-package.sh) | Assembles `dist/zabbix-postfix-install/` (and optional `.tar.gz`) with binaries, agent config, installer, and Zabbix template. |

### How the pieces connect during a poll

1. **Every 1 min** — Zabbix polls `postfix.update_data` → agent runs `zabbix_postfix_passive.sh` → `pflogsumm` parses the last 5 minutes of logs → cached stats file is updated.
2. **Every 3 min** — Zabbix polls `postfix[received]`, `postfix[delivered]`, `postfix[rejected]`, etc. → agent runs `zabbix_postfix_passive.sh <metric>` → script reads the cached value from the stats file and returns the integer.
3. **Every 3 min** — Zabbix polls `postfix.pfmailq` → agent runs `pflogsumm check-mailq --zabbix` → returns current queue size.
4. **Every 1 min** — Zabbix polls `net.tcp.service[smtp]` (built-in agent check, no zabbix-postfix file) → confirms Postfix is accepting SMTP connections.

For installation steps and troubleshooting, see [HOWTO.md](HOWTO.md).