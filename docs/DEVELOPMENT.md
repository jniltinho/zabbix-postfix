# DEVELOPMENT — zabbix-postfix

Technical reference for contributors and advanced users.

---

## Architecture

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

### Update cycle

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

Metrics are **cumulative** — they grow over time. Zabbix graphs the rate of change,
not the raw total.

### UserParameters

```
UserParameter=postfix.pfmailq,/usr/local/bin/check_mailq
UserParameter=postfix[*],sudo /usr/local/sbin/zabbix_postfix_passive.sh $1
UserParameter=postfix.update_data,sudo /usr/local/sbin/zabbix_postfix_passive.sh
```

### Runtime files

| File | Purpose |
|------|---------|
| `/tmp/zabbix-postfix-passive-offset.dat` | Where `pygtail` stopped reading last time |
| `/tmp/zabbix-postfix-passive-statsfile.dat` | Accumulated counters (`key;value` per line) |

Do not delete these while the agent is running.

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

---

## Project structure

```
zabbix-postfix/
├── pygtail/                    Go module — reads log files incrementally
├── pflogsumm/                  Go module — parses Postfix logs, outputs --zabbix key=value
├── check_mailq/                Go module — returns live queue depth as integer
├── zabbix_postfix_passive.sh   Main passive check script (orchestrates the three binaries)
├── zabbix_postfix_passive.conf Zabbix agent UserParameter definitions
├── template_postfix_passive.xml Zabbix template (14 items, 4 graphs, 4 triggers)
├── Makefile                    Build, test, and package orchestration
├── docs/
│   ├── Dockerfile              Multi-stage build: binaries → packages → export
│   ├── HOWTO.md                End-user install guide
│   └── DEVELOPMENT.md          This file
├── scripts/
│   ├── build-binaries-docker.sh  Build Go binaries via Docker
│   ├── build-packages-docker.sh  Build .deb/.rpm/.tar.gz via Docker
│   ├── build-package.sh          fpm wrapper (called by Makefile and Docker)
│   ├── make-install-package.sh   Build the classic install tarball
│   ├── configure_paths.sh        Patch installed scripts to use custom paths
│   └── pkg/
│       ├── postinst            dpkg/rpm post-install hook
│       ├── prerm               dpkg/rpm pre-remove hook
│       └── install.sh          Post-extract installer for .tar.gz
└── .github/workflows/
    └── release.yml             CI: build → package → publish release artifacts
```

---

## Building

### Prerequisites

- Go ≥ 1.26.4
- UPX (`apt install upx-ucl` / `dnf install upx`)
- `make`
- Docker (for the Docker-based paths)
- `fpm` (`gem install fpm`) + `rpm` — only for local package builds

### Build Go binaries

```bash
make build
```

Outputs: `pygtail/dist/pygtail`, `pflogsumm/dist/pflogsumm`, `check_mailq/dist/check_mailq`

### Build via Docker (no local toolchain needed)

Binaries only:

```bash
bash scripts/build-binaries-docker.sh
# copies binaries to each module's dist/ directory
```

All packages (.deb, .rpm, .tar.gz) in one step:

```bash
bash scripts/build-packages-docker.sh [VERSION]
# writes to dist/
```

### Build packages locally (requires Go + UPX + fpm)

```bash
make deb   # dist/zabbix-postfix_<version>_amd64.deb
make rpm   # dist/zabbix-postfix-<version>-1.x86_64.rpm
make pkg   # all three: deb + rpm + tar.gz
```

### Dockerfile stages

| Stage | Description |
|-------|-------------|
| `builder` | Compiles Go binaries with UPX compression |
| `packager` | Runs fpm to build .deb, .rpm, and .tar.gz |
| `export-bins` | Scratch image exporting only the three binaries |
| `export-pkg` | Scratch image exporting .deb, .rpm, and .tar.gz |

```bash
# Export binaries only
docker build -f docs/Dockerfile --target export-bins -o dist/docker-build .

# Export packages only
docker build -f docs/Dockerfile --target export-pkg --build-arg VERSION=1.0.0 -o dist/ .
```

---

## Testing

### Unit tests

```bash
make test
```

### Integration tests (Docker)

```bash
make build
docker build -f Dockerfile.test-passive -t zabbix-postfix-test .
docker run --rm zabbix-postfix-test
# Expected: Results: 19 passed, 0 failed
```

### Manual test on a live agent

```bash
sudo zabbix_agent2 -t postfix.update_data
sudo zabbix_agent2 -t 'postfix[received]'
sudo zabbix_agent2 -t postfix.pfmailq
```

---

## Release process

Releases are triggered by pushing a version tag:

```bash
git tag v1.2.3
git push origin v1.2.3
```

The CI pipeline (`.github/workflows/release.yml`) then:

1. Builds Go binaries with Go + UPX on the runner
2. Packages individual binary tarballs (`pygtail_<v>_linux_amd64.tar.gz`, etc.)
3. Builds the classic install tarball via `make-install-package.sh`
4. Builds `.deb`, `.rpm`, and `.tar.gz` via Docker (`export-pkg` stage)
5. Publishes a GitHub release with all seven artifacts

### Release artifacts

| Artifact | Description |
|----------|-------------|
| `zabbix-postfix_<v>_amd64.deb` | Debian/Ubuntu package |
| `zabbix-postfix-<v>-1.x86_64.rpm` | RHEL/CentOS/Rocky package |
| `zabbix-postfix_<v>_pkg_linux_amd64.tar.gz` | Portable package with `install.sh` |
| `zabbix-postfix_<v>_linux_amd64.tar.gz` | Classic install package |
| `pygtail_<v>_linux_amd64.tar.gz` | Individual binary |
| `pflogsumm_<v>_linux_amd64.tar.gz` | Individual binary |
| `check_mailq_<v>_linux_amd64.tar.gz` | Individual binary |

---

## Advanced configuration

### Custom binary paths

The `.tar.gz` installer supports `--bin-dir` and `--script-dir`:

```bash
mkdir -p /tmp/zabbix-postfix
tar -xzf /tmp/zabbix-postfix_*_pkg_linux_amd64.tar.gz -C /tmp/zabbix-postfix
sudo bash /tmp/zabbix-postfix/usr/share/zabbix-postfix/install.sh \
  --bin-dir /opt/zabbix_bin \
  --script-dir /opt/zabbix_bin
```

The installer:
- Copies binaries and script to the specified directories
- Generates `zabbix_postfix_passive.conf` with the correct paths
- Updates sudoers with the correct script path
- Detects and configures the Zabbix agent conf directory

To patch an existing installation instead:

```bash
sudo bash scripts/configure_paths.sh \
  --bin-dir /opt/zabbix_bin \
  --script-dir /opt/zabbix_bin
```

### Environment variable overrides

Override paths at runtime without reinstalling — useful for testing:

```bash
# Use a different log file (e.g. rotated)
sudo MAILLOG=/var/log/mail.log.1 /usr/local/sbin/zabbix_postfix_passive.sh

# Use different binaries
ZABBIX_POSTFIX_PFLOGSUMM=/tmp/pflogsumm_new \
ZABBIX_POSTFIX_PYGTAIL=/tmp/pygtail_new \
  sudo -E /usr/local/sbin/zabbix_postfix_passive.sh
```

### Manual installation (without a package manager)

```bash
# 1. Install binaries
sudo install -m 0755 pygtail/dist/pygtail pflogsumm/dist/pflogsumm \
  check_mailq/dist/check_mailq /usr/local/bin/

# 2. Install passive script
sudo install -m 0755 zabbix_postfix_passive.sh /usr/local/sbin/

# 3. Install agent UserParameter conf
sudo install -m 0644 zabbix_postfix_passive.conf \
  /etc/zabbix/zabbix_agent2.d/     # or zabbix_agentd.conf.d/

# 4. Add sudoers entry
echo 'zabbix ALL=(ALL) NOPASSWD: /usr/local/sbin/zabbix_postfix_passive.sh' \
  | sudo EDITOR='tee -a' visudo

# 5. Restart agent
sudo systemctl restart zabbix-agent2
```

---

## Uninstall

**.deb**
```bash
sudo dpkg -r zabbix-postfix
```

**.rpm**
```bash
sudo rpm -e zabbix-postfix
```

**Manual**
```bash
sudo rm -f /usr/local/bin/pygtail \
           /usr/local/bin/pflogsumm \
           /usr/local/bin/check_mailq \
           /usr/local/sbin/zabbix_postfix_passive.sh \
           /etc/zabbix/zabbix_agent2.d/zabbix_postfix_passive.conf
sudo sed -i '/zabbix_postfix_passive/d' /etc/sudoers
sudo systemctl restart zabbix-agent2
# Optional: remove runtime files
sudo rm -f /tmp/zabbix-postfix-passive-*.dat
```

---

## Migration from Python/Perl

If you were using the original `pygtail.py` and Perl `pflogsumm`:

| Item | Action |
|------|--------|
| Offset file | No change — same format |
| Stats file | No change — same `key;value` format |
| Zabbix template | No re-import needed — same item keys |
| pflogsumm | Go binary outputs identical `--zabbix` key=value format |
| Old packages | `apt remove pflogsumm` is optional |
