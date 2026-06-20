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
                                             │       └─ pflogsumm --last 5m (parses and counts)
                         postfix[received]   ──▶ reads saved counter → integer
                         postfix[delivered]  ──▶ reads saved counter → integer
                         postfix.pfmailq     ──▶ pflogsumm check-mailq --zabbix → queue depth
```

### Update cycle

```
Every 1 minute — postfix.update_data
    pflogsumm --zabbix --last 5m parses the last 5 minutes of mail.log:
        received=42
        delivered=38
        bytes_received=1048576
        ...
        ▼
    zabbix_postfix_passive.sh writes the stats file (absolute totals, rolling window):
        received=42
        delivered=38
        ...

Every 3 minutes — postfix[received], postfix[delivered], ...
    zabbix_postfix_passive.sh reads one line from the stats file → integer → Zabbix
```

Metrics represent the **last 5 minutes** of activity. Zabbix graphs these as a
rolling time-series. Use `--reset` to clear the cache if stale data appears.

### UserParameters

```
UserParameter=postfix.pfmailq,/opt/zabbix_postfix/pflogsumm check-mailq --zabbix
UserParameter=postfix[*],sudo /opt/zabbix_postfix/zabbix_postfix_passive.sh $1
UserParameter=postfix.update_data,sudo /opt/zabbix_postfix/zabbix_postfix_passive.sh
```

### Runtime files

| File | Purpose |
|------|---------|
| `/tmp/zabbix-postfix-passive-statsfile.dat` | Last 5-minute counters (`key=value` per line) |

Delete this file to force a fresh poll on the next `postfix.update_data` call
(equivalent to running `zabbix_postfix_passive.sh --reset`).

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
├── pflogsumm/                  Go module — parses logs, outputs --zabbix counters,
│   ├── cmd/                      includes check-mailq subcommand for queue depth
│   │   ├── root.go
│   │   └── checkmailq.go
│   ├── internal/mailq/         Queue runner + parser (used by check-mailq)
│   └── pkg/parser/             Log parser (ParseFiltered, ParseLastN)
├── zabbix_postfix_passive.sh   Main passive check script (orchestrates pflogsumm)
├── zabbix_postfix_passive.conf Zabbix agent UserParameter definitions
├── template_postfix_passive.xml Zabbix template (14 items, 4 graphs, 4 triggers)
├── Makefile                    Build, test, and package orchestration
├── docs/
│   ├── Dockerfile              Multi-stage build: binaries → packages → export
│   ├── HOWTO.md                End-user install guide
│   └── DEVELOPMENT.md          This file
├── scripts/
│   ├── build-binaries-docker.sh  Build Go binary via Docker
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

Output: `pflogsumm/dist/pflogsumm`

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
# Expected: Results: 15 passed, 0 failed
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

1. Builds the Go binary with Go + UPX on the runner
2. Packages the individual binary tarball (`pflogsumm_<v>_linux_amd64.tar.gz`)
3. Builds the classic install tarball via `make-install-package.sh`
4. Builds `.deb`, `.rpm`, and `.tar.gz` via Docker (`export-pkg` stage)
5. Publishes a GitHub release with all five artifacts

### Release artifacts

| Artifact | Description |
|----------|-------------|
| `zabbix-postfix_<v>_amd64.deb` | Debian/Ubuntu package |
| `zabbix-postfix-<v>-1.x86_64.rpm` | RHEL/CentOS/Rocky package |
| `zabbix-postfix_<v>_pkg_linux_amd64.tar.gz` | Portable package with `install.sh` |
| `zabbix-postfix_<v>_linux_amd64.tar.gz` | Classic install package |
| `pflogsumm_<v>_linux_amd64.tar.gz` | Individual binary |

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
sudo MAILLOG=/var/log/mail.log.1 /opt/zabbix_postfix/zabbix_postfix_passive.sh

# Use a different pflogsumm binary
ZABBIX_POSTFIX_PFLOGSUMM=/tmp/pflogsumm_new \
  sudo -E /opt/zabbix_postfix/zabbix_postfix_passive.sh
```

### Manual installation (without a package manager)

```bash
# 1. Install binary
sudo install -m 0755 pflogsumm/dist/pflogsumm /opt/zabbix_postfix/

# 2. Install passive script
sudo install -m 0755 zabbix_postfix_passive.sh /opt/zabbix_postfix/

# 3. Install agent UserParameter conf
sudo install -m 0644 zabbix_postfix_passive.conf \
  /etc/zabbix/zabbix_agent2.d/     # or zabbix_agentd.conf.d/

# 4. Add sudoers entry
echo 'zabbix ALL=(ALL) NOPASSWD: /opt/zabbix_postfix/zabbix_postfix_passive.sh' \
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
sudo rm -f /opt/zabbix_postfix/pflogsumm \
           /opt/zabbix_postfix/zabbix_postfix_passive.sh \
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
| Offset file | No longer needed — delete `/tmp/zabbix-postfix-passive-offset.dat` |
| Stats file | Format changed to `key=value`; old `key;value` file will be overwritten on first poll |
| Zabbix template | No re-import needed — same item keys |
| pflogsumm | Go binary outputs identical `--zabbix` key=value format |
| Old packages | `apt remove pflogsumm` is optional |
