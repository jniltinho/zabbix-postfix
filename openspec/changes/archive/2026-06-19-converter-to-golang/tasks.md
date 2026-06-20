## 1. Repo Scaffolding

- [x] 1.1 Create root `.gitignore` with standard Go entries plus `.claude/`, `.aider*`, `*.offset`, `testdata/*.log` (keep `openspec/` versioned)
- [x] 1.2 Create root `.golangci.yml` with baseline lint config
- [x] 1.3 Create root `Makefile` with targets: `build` (`CGO_ENABLED=0`, `-ldflags="-s -w"`), `test`, `lint`, `install`, `clean`, `fetch-testdata` (scp from HOST)
- [x] 1.4 Create root `README.md` describing the project, the three modules, prerequisites (Go ≥1.21), build+install steps, and note that Zabbix passive integration is a follow-up change

## 2. pygtail Module

- [x] 2.1 `mkdir pygtail/` and run `go mod init` with canonical module path; add `cobra` dependency
- [x] 2.2 Create `pygtail/internal/offset/offset.go` — read/write offset file (`<inode>\n<byteoffset>\n` format)
- [x] 2.3 Create `pygtail/internal/reader/reader.go` — incremental read and log rotation detection
- [x] 2.4 Implement log rotation candidate search: `.0`, `.1`, `.1.gz`, dateext glob patterns, TimedRotatingFileHandler pattern (matching pygtail.py priority order)
- [x] 2.5 Implement copytruncate support (default on) with `--no-copytruncate` flag
- [x] 2.6 Add transparent `.gz` decompression using `compress/gzip` for rotated compressed logs
- [x] 2.7 Create `pygtail/cmd/root.go` — Cobra root command with `--offset-file`/`-o` flag and positional `<logfile>` argument
- [x] 2.8 Create `pygtail/main.go` entry point
- [x] 2.9 Add godoc comments to all exported symbols in `offset` and `reader` packages
- [x] 2.10 Create `pygtail/Makefile` with `build`, `test`, `install` targets; binary name `pygtail`
- [x] 2.11 Create `pygtail/README.md` with usage, flags, log rotation, and note on future passive script integration
- [x] 2.12 Add `--version` flag printing `pygtail version <semver>`

## 3. pflogsumm Module

- [x] 3.1 `mkdir pflogsumm/` and run `go mod init` with canonical module path; add `cobra` dependency
- [x] 3.2 Define `pflogsumm/pkg/parser/metrics.go` — `Metrics` struct with all 11 fields
- [x] 3.3 Create `pflogsumm/pkg/parser/parser.go` — `Parse(r io.Reader) (Metrics, error)` parsing Postfix log lines using regex; handle byte suffix conversion (k→×1024, m→×1048576, g→×1073741824)
- [x] 3.4 Create `pflogsumm/internal/formatter/formatter.go` — `Format(m Metrics, format string) string` implementing `keyvalue` (default), `json`, and `summary` formats
- [x] 3.5 Create `pflogsumm/cmd/root.go` — Cobra root command accepting stdin or file arg; `--format` flag; passive and active compatibility flags accepted and silently ignored
- [x] 3.6 Create `pflogsumm/main.go` entry point
- [x] 3.7 Add godoc comments to all exported symbols in `pkg/parser` and `internal/formatter`
- [x] 3.8 Create `pflogsumm/Makefile` with `build`, `test`, `install` targets; binary name `pflogsumm`
- [x] 3.9 Create `pflogsumm/README.md` with usage (stdin, file, `--format` options), output reference (11 keys), golden test note
- [x] 3.10 Add `--version` flag printing `pflogsumm version <semver>`

## 4. check_mailq Module

- [x] 4.1 `mkdir check_mailq/` and run `go mod init` with canonical module path; add `cobra` dependency
- [x] 4.2 Create `check_mailq/pkg/mailq/parser.go` — `ParseOutput(output string) (int, error)` counting lines matching `^[0-9A-Z]` (excluding "Mail queue is empty")
- [x] 4.3 Create `check_mailq/internal/runner/runner.go` — `Run(mailqPath string, timeout time.Duration) (string, error)` executing `mailq` subprocess (default timeout 10s)
- [x] 4.4 Create `check_mailq/cmd/root.go` — Cobra root command with `--mailq-path` and `--timeout` flags
- [x] 4.5 Create `check_mailq/main.go` entry point
- [x] 4.6 Add godoc comments to all exported symbols in `pkg/mailq`
- [x] 4.7 Create `check_mailq/Makefile` with `build`, `test`, `install` targets; binary name `check_mailq`
- [x] 4.8 Create `check_mailq/README.md` with usage, planned UserParameter example, parity note with grep pipeline
- [x] 4.9 Add `--version` flag printing `check_mailq version <semver>`

## 5. Tests

- [x] 5.1 Create `pygtail/testdata/` with a sample `mail.log` fixture (can be empty initially; `make fetch-testdata` populates it)
- [x] 5.2 Write `pygtail/internal/offset/offset_test.go` — unit tests for read/write round-trip and compatibility with pygtail.py format
- [x] 5.3 Write `pygtail/internal/reader/reader_test.go` — unit tests for: first-run full read, incremental read, copytruncate shrink, rotation with `.1`, rotation with `.1.gz`
- [x] 5.4 Write `pygtail/reader_integration_test.go` (build tag `integration`) — reads from `testdata/mail.log` and verifies non-empty output
- [x] 5.5 Create `pflogsumm/testdata/` directory
- [x] 5.6 Write `pflogsumm/pkg/parser/parser_test.go` — unit tests for: empty log, received/delivered counts, byte suffix conversion (k, m, g), malformed lines ignored
- [x] 5.7 Write `pflogsumm/internal/formatter/formatter_test.go` — unit tests for keyvalue, JSON, and summary formats
- [x] 5.8 Write `pflogsumm/golden_test.go` (build tag `integration`) — compare Go vs bundled `pflogsumm.pl` on `testdata/mail.log` for all 11 metrics
- [x] 5.9 Create `check_mailq/testdata/` with `mailq_output_nonempty.txt` and `mailq_output_empty.txt` fixtures
- [x] 5.10 Write `check_mailq/pkg/mailq/parser_test.go` — unit tests matching `grep -v "Mail queue is empty" | grep -c '^[0-9A-Z]'` behavior
- [x] 5.11 Write `check_mailq/internal/runner/runner_test.go` — mock subprocess test verifying timeout kills subprocess and returns error
- [x] 5.12 Add `make fetch-testdata HOST=<host>` target: `scp $(HOST):/var/log/mail.log pygtail/testdata/mail.log pflogsumm/testdata/mail.log`

## 6. Documentation

- [x] 6.1 Ensure module READMEs cover: Overview, Build, Install, Usage, Testing, and "Zabbix integration (follow-up change)"
- [x] 6.2 Update root `README.md` with links to module READMEs and a "Migration roadmap" section: (1) validate Go binaries, (2) update passive script, (3) future plugin
- [x] 6.3 Ensure all godoc comments follow Go doc conventions (first sentence is a complete statement starting with the symbol name)

## 7. Integration & Verification

- [x] 7.1 Run `make build` from repo root and verify all three binaries are produced
- [x] 7.2 Run `make test` and verify all unit tests pass
- [x] 7.3 Fetch `mail.log` from mx01 via `make fetch-testdata HOST=mx01` and run integration tests: `go test -tags integration ./...` in pygtail and pflogsumm
- [x] 7.4 Run golden test: compare `pygtail -o /tmp/t.offset testdata/mail.log | pflogsumm` metrics against `pygtail.py | pflogsumm.pl` on the same input; all 11 values must match
- [x] 7.5 Test `check_mailq` locally with fixtures and verify count matches `grep -v "Mail queue is empty" | grep -c '^[0-9A-Z]'` on the same fixture output
- [ ] 7.6 Run `make install` and verify binaries land in `/usr/local/bin/` and are executable
- [ ] 7.7 Run `make lint` and fix any reported issues
