## ADDED Requirements

### Requirement: Count messages in the Postfix mail queue
The system SHALL execute `mailq` as a subprocess, count the number of queued messages, and write the integer count to stdout. An empty queue SHALL output `0`.

Counting SHALL match the current Zabbix UserParameter pipeline: exclude lines containing "Mail queue is empty", then count lines matching `^[0-9A-Z]`.

#### Scenario: Non-empty queue
- **WHEN** `check_mailq` is run and `mailq` reports queued messages
- **THEN** stdout contains the integer count matching the grep pipeline behavior

#### Scenario: Empty queue
- **WHEN** `check_mailq` is run and `mailq` reports "Mail queue is empty"
- **THEN** stdout contains `0`

#### Scenario: Count matches current UserParameter behavior
- **WHEN** the result of `check_mailq` is compared with `mailq | grep -v "Mail queue is empty" | grep -c '^[0-9A-Z]'`
- **THEN** the counts are equal

### Requirement: Exit code contract
The system SHALL exit 0 on success (even if queue count is 0). It SHALL exit 1 if `mailq` cannot be executed or returns a non-zero exit code.

#### Scenario: mailq not found
- **WHEN** `mailq` binary is not in PATH
- **THEN** stderr contains an error message and exit code is 1

#### Scenario: mailq timeout
- **WHEN** `mailq` does not respond within 10 seconds
- **THEN** the subprocess is killed, stderr contains a timeout error, and exit code is 1

### Requirement: Configurable mailq binary path
The system SHALL support a `--mailq-path` flag (default: `mailq`, resolved via PATH) allowing operators to specify the full path (e.g., `/usr/sbin/mailq`).

#### Scenario: Custom mailq path
- **WHEN** `check_mailq --mailq-path /usr/sbin/mailq` is run
- **THEN** `/usr/sbin/mailq` is executed instead of searching PATH

### Requirement: Zabbix UserParameter compatibility (follow-up wiring)
The binary output (integer on stdout, nothing else) SHALL be directly usable as a Zabbix UserParameter value once the follow-up change updates `zabbix_postfix_passive.conf`:
```
UserParameter=postfix.pfmailq,/usr/local/bin/check_mailq
```

#### Scenario: Zabbix UserParameter usage
- **WHEN** Zabbix agent calls `check_mailq` via UserParameter
- **THEN** stdout contains only the integer count with no trailing spaces or extra lines

### Requirement: Module structure and exportable parser
All exported functions SHALL have godoc comments. Queue-parsing logic SHALL live in `check_mailq/pkg/mailq` (importable by future plugin). Subprocess execution stays in `internal/runner` or equivalent.

#### Scenario: Queue parser unit testable without subprocess
- **WHEN** `mailq.ParseOutput(output string) (int, error)` is called with a sample mailq output string
- **THEN** it returns the correct integer count without invoking any subprocess
