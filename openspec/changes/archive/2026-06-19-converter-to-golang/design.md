## Context

The current Postfix monitoring stack uses three external runtimes:
- **Python**: `pygtail.py` (log offset tracker, bundled in repo)
- **Perl**: `/usr/sbin/pflogsumm` or bundled `pflogsumm.pl` (system package, OS-dependent path)
- **Shell + mailq**: inline `mailq | grep` pipeline in Zabbix UserParameter

Each monitored host must have Python 3, Perl, pflogsumm, bc, and the correct system paths configured. This creates dependency friction and makes the stack fragile across distros (Debian uses `pflogsumm`, RHEL uses `postfix-perl-scripts`). The goal is three self-contained Go binaries that can be compiled once and deployed as static executables.

The passive Zabbix flow (`zabbix_postfix_passive.sh`) orchestrates stateful accumulation: `pygtail | pflogsumm` on update, stats file on disk, single-metric reads on poll. That orchestration will be migrated in a **follow-up change**; this change delivers only the underlying Go tools.

## Goals / Non-Goals

**Goals:**
- Functional parity with the original tools for the 11 metrics used by the Zabbix scripts
- Each binary is a standalone Go module (`go build` produces one binary per module)
- CLI flags match the originals closely enough for the future passive script migration
- `pflogsumm-go` default output is `key=value` (machine-parseable) for the updated passive script
- Exportable parser packages (`pkg/`) for reuse by a future `check_postfix` plugin
- Comprehensive godoc on all exported symbols
- Test fixtures per module; golden tests comparing Go vs Perl output on the same log slice
- Makefile targets: `build`, `test`, `install`, `clean`, `lint`
- Standard Go `.gitignore` + exclusion of `.claude/`, `.aider*` (keep `openspec/` versioned)

**Non-Goals:**
- Full 1:1 replication of every pflogsumm flag (only flags used by the Zabbix scripts are required)
- Windows support
- Updating `zabbix_postfix_passive.sh`, `zabbix_postfix.sh`, Zabbix `.conf`, sudoers, or XML templates in this change
- End-to-end Zabbix passive integration (deferred to follow-up change after binaries are validated)
- Building the `check_postfix` Zabbix plugin (future change)

## Decisions

### D1: One Go module per tool, not a monorepo workspace (yet)

**Decision**: Each tool lives in its own directory with its own `go.mod` (`pygtail/go.mod`, `pflogsumm/go.mod`, `check_mailq/go.mod`). A root `Makefile` orchestrates all three. No `go.work` in this change.

**Rationale**: Keeps modules independently deployable. A user who only wants `pygtail` doesn't pull in pflogsumm's dependencies. Matches the requirement "o nome dos módulos são o nome do script." Add `go.work` later if modules need to share code at dev time.

**Alternative considered**: Single module with sub-packages — rejected because it forces users to install everything together and complicates `go install` for individual tools.

### D2: Cobra for CLI; Viper optional per module

**Decision**: Use `github.com/spf13/cobra` for command structure in all three modules. Use `github.com/spf13/viper` only where env-var binding adds value (e.g., `check_mailq` timeout/path overrides).

**Rationale**: Cobra is standard for CLI structure and `--version`. Viper is useful but not mandatory for one-shot UserParameter-style invocations.

**Alternative considered**: `flag` stdlib only — rejected; too limited for future flag growth.

### D3: pygtail-go — offset file uses inode + byte offset (same format as pygtail.py)

**Decision**: Offset file format: two lines — `<inode>\n<offset>\n`. Identical to `pygtail.py` v0.11.1. Copytruncate support enabled by default (same as pygtail.py).

**Rationale**: Allows zero-downtime cutover; the Go binary can continue where the Python script left off without losing position.

**Log rotation detection**: compare current inode vs stored inode; if different, check for rotated files in order: `.0`, `.1`, `.1.gz`, dateext patterns, TimedRotatingFileHandler pattern. Same priority order as pygtail.py.

### D4: pflogsumm-go — reads from stdin or file, outputs key=value lines (default)

**Decision**: Accept log data on stdin (pipe-friendly) or as a positional file argument. Default output: one metric per line as `<key>=<value>` (e.g. `received=142`). A `--format summary` flag produces human-readable text similar to Perl pflogsumm for debugging.

**Rationale**: The follow-up passive script migration will parse `key=value` directly (`grep '^received=' | cut -d= -f2`), replacing fragile grep on Perl summary text. JSON is available via `--format json` for the future plugin.

**Note**: The current `zabbix_postfix_passive.sh` is **not** compatible with this default until migrated — that is intentional and scoped to the follow-up change.

### D5: check_mailq-go — shells out to `mailq`

**Decision**: Execute `mailq` as a subprocess, parse output, return count to stdout. Counting algorithm matches the current UserParameter pipeline: lines matching `^[0-9A-Z]` after excluding "Mail queue is empty".

**Rationale**: `mailq` is a Postfix tool; reimplementing its queue-reading logic requires direct Postfix queue directory access and root/postfix group permissions. Shelling out is simpler, safer, and preserves behavioral parity with the existing grep-based UserParameter (including its edge cases).

**Subprocess**: `exec.Command("mailq")` with a 10-second timeout.

### D6: Exportable packages in `pkg/`, not `internal/`

**Decision**: Shared logic that a future `check_postfix` plugin must import lives under `pkg/` (e.g., `pflogsumm/pkg/parser`, `check_mailq/pkg/mailq`). Module-private helpers stay in `internal/`.

**Rationale**: Go `internal/` packages cannot be imported from outside the module tree; the future plugin needs a public API.

### D7: Testing strategy — testdata/ + unit tests + golden tests

**Decision**: Each module has a `testdata/` directory. Integration tests use `//go:build integration`. Golden tests compare `pflogsumm-go` output against bundled `pflogsumm.pl` on the same input.

**Unit tests** mock file I/O and test parsing logic independently.
**Integration tests** run against real `mail.log` files — manually copied or fetched via `make fetch-testdata HOST=mx01`.

### D8: Makefile structure

Root `Makefile` delegates to each module:
```
build:     CGO_ENABLED=0 go build -ldflags="-s -w" in each module dir
test:      go test ./... in each module dir
install:   installs binaries to /usr/local/bin/
clean:     removes build artifacts
fetch-testdata: scp $(HOST):/var/log/mail.log → pygtail/ and pflogsumm/ testdata/
lint:      golangci-lint run (requires .golangci.yml)
```

## Risks / Trade-offs

- **pflogsumm parsing complexity** → pflogsumm's Perl source is ~3k lines; the Go version only needs to match the subset of output fields used by the Zabbix scripts. Risk: edge cases in unusual mail.log formats. Mitigation: golden tests against `pflogsumm.pl`; integration tests with real `mail.log` files.
- **Passive script not updated yet** → Go binaries cannot be wired into production Zabbix until the follow-up change. Mitigation: validate binaries manually (`make test`, task 7.4) before migrating the script.
- **pygtail inode handling on tmpfs** → `/tmp`-mounted systems may reuse inodes. Mitigation: same limitation exists in pygtail.py; document it.
- **mailq permissions** → `mailq` requires the calling user to be in `postfix` group or run via sudo. Mitigation: unchanged until follow-up change updates sudoers.
- **Go binary size** → statically linked Go binaries are ~5–8 MB each vs ~50 KB Python script. Mitigation: acceptable for server-side monitoring tools; use `CGO_ENABLED=0` + `ldflags="-s -w"` to minimize.

## Open Questions

- Should `pflogsumm-go` support gzip-compressed rotated logs as **input** (like the original Perl tool)? Likely yes; defer to spec.
- Should the `Makefile` include a `deb` or `rpm` packaging target? Out of scope for now but easy to add.
- Module import path: use a canonical path (e.g., `github.com/rafael747/zabbix-postfix/pygtail`) instead of bare `pygtail`? Decide before first release tag.

## Follow-up Change (out of scope here)

Update `zabbix_postfix_passive.sh` to:
1. Call `/usr/local/bin/pygtail` and `/usr/local/bin/pflogsumm` instead of Python/Perl
2. Parse `key=value` output on update (replace grep/awk on Perl summary text)
3. Keep stats-file accumulation logic; update `zabbix_postfix_passive.conf` and sudoers paths
