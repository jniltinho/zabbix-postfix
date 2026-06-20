## ADDED Requirements

### Requirement: Read new log lines since last offset
The system SHALL read only lines appended to a log file since the previous run, determined by a stored byte offset and inode, and write those lines to stdout. On first run (no offset file), it SHALL read from the beginning of the file.

#### Scenario: First run with no offset file
- **WHEN** `pygtail <logfile>` is run and no offset file exists
- **THEN** all lines in the log file are written to stdout and an offset file is created recording the current inode and byte position

#### Scenario: Subsequent run with offset file
- **WHEN** `pygtail <logfile>` is run and a valid offset file exists
- **THEN** only lines written after the previous offset are written to stdout and the offset file is updated

#### Scenario: No new lines since last run
- **WHEN** `pygtail <logfile>` is run and the file has not grown since last offset
- **THEN** nothing is written to stdout and exit code is 0

### Requirement: Offset file format compatible with pygtail.py
The offset file SHALL contain exactly two lines: the inode number on line 1, the byte offset on line 2. This matches the format written by `pygtail.py` v0.11.1 so cutover from Python to Go loses no position.

#### Scenario: Offset file format
- **WHEN** the offset file is written after reading a log file
- **THEN** it contains `<inode>\n<byteoffset>\n` with no other content

#### Scenario: Inheriting Python pygtail offset
- **WHEN** an offset file previously written by `pygtail.py` exists
- **THEN** `pygtail-go` reads the correct inode and offset from it without error

### Requirement: Custom offset file path via flag
The system SHALL accept a `--offset-file` / `-o` flag specifying the path to the offset file. Default SHALL be `<logfile>.offset`.

#### Scenario: Custom offset file path
- **WHEN** `pygtail -o /tmp/my.offset <logfile>` is run
- **THEN** the offset is read from and written to `/tmp/my.offset`

### Requirement: Copytruncate log rotation support (default enabled)
The system SHALL support copytruncate-style rotation by default (same as pygtail.py). When the file inode is unchanged but size shrinks below the stored offset, it SHALL treat the file as rotated and continue reading from the beginning. A `--no-copytruncate` flag SHALL disable this behavior and emit a stderr warning instead.

#### Scenario: Copytruncate shrink detected
- **WHEN** logrotate copytruncates the file (same inode, smaller size than stored offset)
- **THEN** reading continues from the beginning of the current file without error

#### Scenario: Copytruncate disabled
- **WHEN** `pygtail --no-copytruncate <logfile>` is run and the file shrinks below the stored offset
- **THEN** a warning is written to stderr (same behavior as pygtail.py)

### Requirement: Log rotation detection
The system SHALL detect that a log file has been rotated when the current inode differs from the stored inode. On rotation detection it SHALL:
1. Read remaining unread lines from the rotated file (if locatable)
2. Then read from the beginning of the current (new) log file

Rotation candidate search order:
1. `<logfile>.0` (savelog style)
2. `<logfile>.1` (logrotate with delaycompress)
3. `<logfile>.1.gz` (logrotate without delaycompress)
4. Dateext patterns: `<logfile>-YYYYMMDD`, `<logfile>-YYYYMMDD.gz`, `<logfile>-YYYYMMDD-NNNNNNNNNN`, `<logfile>-YYYYMMDD-NNNNNNNNNN.gz`
5. TimedRotatingFileHandler pattern: `<logfile>.YYYY-MM-DD`

#### Scenario: Log rotated to .1
- **WHEN** `mail.log` has been rotated to `mail.log.1` and `mail.log` is a new empty file
- **THEN** unread lines from `mail.log.1` are emitted first, then lines from the new `mail.log`

#### Scenario: Log rotated with gzip compression
- **WHEN** `mail.log` has been rotated to `mail.log.1.gz`
- **THEN** the compressed file is transparently decompressed and unread lines are emitted

#### Scenario: No rotated file found
- **WHEN** the inode has changed and no candidate rotated file is found
- **THEN** reading continues from the beginning of the current file and a warning is written to stderr

### Requirement: Graceful handling of unreadable files
The system SHALL exit with code 1 and write a descriptive error to stderr if the log file does not exist or is not readable.

#### Scenario: Log file not found
- **WHEN** `pygtail /nonexistent/mail.log` is run
- **THEN** stderr contains an error message, stdout is empty, and exit code is 1

### Requirement: CLI invocation compatible with pygtail.py
The binary SHALL be invocable as `pygtail [--offset-file|-o <path>] <logfile>` so the future passive script needs only change the binary path (not flags).

#### Scenario: Drop-in replacement for future passive script
- **WHEN** a script calls `pygtail -o /tmp/offset.dat /var/log/mail.log` (replacing `pygtail.py`)
- **THEN** output and offset file behavior are identical to pygtail.py v0.11.1
