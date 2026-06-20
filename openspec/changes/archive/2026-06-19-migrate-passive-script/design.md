## Context

After `converter-to-golang`, three Go binaries exist at `/usr/local/bin/`:
- `pygtail` — incremental log reader (offset file compatible with pygtail.py)
- `pflogsumm` — Postfix log parser, default output `key=value`
- `check_mailq` — queue depth counter

The passive Zabbix flow today (`zabbix_postfix_passive.sh`):
1. **Update** (no args, triggered by `postfix.update_data`): `pygtail | pflogsumm` → parse Perl summary → accumulate in `/tmp/zabbix-postfix-passive-statsfile.dat`
2. **Read** (arg = metric name, triggered by `postfix[received]` etc.): read single value from stats file
3. **Mailq** (separate UserParameter `postfix.pfmailq`): inline grep pipeline

UserParameters in `zabbix_postfix_passive.conf` must keep the same keys so the existing Zabbix template (`template_postfix_passive.xml`) continues working without re-import.

## Goals / Non-Goals

**Goals:**
- Replace Python/Perl invocations with Go binaries inside `zabbix_postfix_passive.sh`
- Parse `pflogsumm` `key=value` output instead of Perl summary text
- Keep stats-file format (`key;value`), offset file path, and UserParameter keys unchanged
- Update installer and README to reflect Go prerequisites
- Point `postfix.pfmailq` to `/usr/local/bin/check_mailq`

**Non-Goals:**
- Rewriting the passive script in Go (bash orchestration is sufficient for now)
- Changing the Zabbix XML template or item keys
- Migrating active mode (`zabbix_postfix.sh` / zabbix_sender) — separate future change
- Removing bundled `pygtail.py` or `pflogsumm.pl` from repo (keep as reference/fallback)

## Decisions

### D1: Keep bash script, change internals only

**Decision**: Update `zabbix_postfix_passive.sh` in place; do not replace with a Go binary in this change.

**Rationale**: The script's value is orchestration + stats accumulation + Zabbix I/O contract. Go binaries already handle the heavy lifting. Minimal diff, same sudoers entry, same install path `/usr/local/sbin/`.

**Alternative considered**: Single Go binary `postfix-passive` — rejected; duplicates stats-file logic already working in bash; can be a later refactor.

### D2: Hardcode Go binary paths with override via env vars

**Decision**: Default paths:
```
PYGTAIL=/usr/local/bin/pygtail
PFLOGSUMM=/usr/local/bin/pflogsumm
```
Allow override via environment variables `ZABBIX_POSTFIX_PYGTAIL` and `ZABBIX_POSTFIX_PFLOGSUMM` for testing.

**Rationale**: Matches current script pattern (fallback chain removed — Go binaries are the only supported path after migration). Env override aids debugging without editing the script.

### D3: Parse key=value on update

**Decision**: Replace `updatevalue()` grep/awk on Perl text with:
```bash
value=$(grep -m1 "^${key}=" "$TEMPFILE" | cut -d= -f2)
```
Byte metrics are already integers in Go output — no k/m/g conversion needed.

**Rationale**: Simpler and more reliable than parsing `"123k bytes received"`. Depends on `pflogsumm-go` default format from `converter-to-golang`.

### D4: Pipeline unchanged except binary paths

**Decision**: Update mode still runs:
```bash
"${PYGTAIL}" -o"${PFOFFSETFILE}" "${MAILLOG}" | \
  "${PFLOGSUMM}" -h 0 -u 0 --no_bounce_detail ... > "${TEMPFILE}" 2>/dev/null
```
Compatibility flags remain (no-ops in Go) for documentation parity.

**Rationale**: Offset file at `/tmp/zabbix-postfix-passive-offset.dat` may already exist from pygtail.py — Go pygtail reads it seamlessly.

### D5: check_mailq via UserParameter only

**Decision**: Change `zabbix_postfix_passive.conf` line:
```
UserParameter=postfix.pfmailq,/usr/local/bin/check_mailq
```
No sudo needed for check_mailq if zabbix user can run mailq (same as current grep pipeline — verify sudoers if mailq requires it).

**Rationale**: Direct binary call; cleaner than shell pipeline. Matches `zabbix-compat` spec from `converter-to-golang`.

### D6: Installer checks Go binaries instead of Python/Perl

**Decision**: Replace python3/pip/pflogsumm/pygtail checks with:
- `/usr/local/bin/pygtail`
- `/usr/local/bin/pflogsumm`
- `/usr/local/bin/check_mailq`

Prompt user to run `make install` from repo if missing.

**Rationale**: Installer should match new deployment model.

## Risks / Trade-offs

- **Go binaries not installed** → script fails at startup with clear ERROR. Mitigation: installer checks + README prerequisites.
- **Existing stats file from Perl era** → format is `key;value` integers; new increments from Go should be compatible. Mitigation: no format change; values continue accumulating.
- **Existing offset file from pygtail.py** → Go pygtail reads same format. Mitigation: documented in converter-to-golang; no reset needed.
- **mailq permissions for check_mailq** → may need sudo or postfix group. Mitigation: document in README; if current grep pipeline works without sudo, check_mailq should too.
- **Typo in original script** (`zaabbix:zabbix` in error message) → fix while editing.

## Migration Plan

1. Deploy Go binaries (`make install` from `converter-to-golang`)
2. Replace `zabbix_postfix_passive.sh` and `.conf` on agent host
3. Restart zabbix-agent
4. Trigger `postfix.update_data`; verify stats file grows
5. Poll `postfix[received]` etc.; compare values with pre-migration baseline over 24h
6. **Rollback**: restore old script + `.conf`; offset/stats files remain compatible

## Open Questions

- Should `check_mailq` run via `sudo` in UserParameter if mailq requires it on some distros? Test on mx01 before finalizing `.conf`.
- Fix installer typo `cp .zabbix_postfix_passive.conf` → `cp zabbix_postfix_passive.conf` while editing?
