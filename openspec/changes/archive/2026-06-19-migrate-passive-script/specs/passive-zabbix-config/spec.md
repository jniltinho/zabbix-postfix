## ADDED Requirements

### Requirement: UserParameter for mail queue uses check_mailq
The passive Zabbix config SHALL define `postfix.pfmailq` calling the Go binary directly:
```
UserParameter=postfix.pfmailq,/usr/local/bin/check_mailq
```

#### Scenario: Mail queue UserParameter
- **WHEN** Zabbix polls `postfix.pfmailq`
- **THEN** the agent executes `/usr/local/bin/check_mailq` and returns an integer count

### Requirement: UserParameters for stats unchanged
The passive Zabbix config SHALL keep existing UserParameter keys for metric reads and updates:
```
UserParameter=postfix[*],sudo /usr/local/sbin/zabbix_postfix_passive.sh $1
UserParameter=postfix.update_data,sudo /usr/local/sbin/zabbix_postfix_passive.sh
```

#### Scenario: Metric read UserParameter
- **WHEN** Zabbix polls `postfix[received]`
- **THEN** the agent runs `sudo /usr/local/sbin/zabbix_postfix_passive.sh received`

#### Scenario: Update UserParameter
- **WHEN** Zabbix polls `postfix.update_data`
- **THEN** the agent runs `sudo /usr/local/sbin/zabbix_postfix_passive.sh` (no args)

### Requirement: Sudoers entry unchanged
Sudoers SHALL grant zabbix passwordless sudo for the passive script only:
```
zabbix ALL=(ALL) NOPASSWD: /usr/local/sbin/zabbix_postfix_passive.sh
```

#### Scenario: Sudoers for passive script
- **WHEN** the installer configures sudoers
- **THEN** only `/usr/local/sbin/zabbix_postfix_passive.sh` is granted, not individual Go binaries

### Requirement: Installer checks Go binaries
The passive installer SHALL verify `/usr/local/bin/pygtail`, `/usr/local/bin/pflogsumm`, and `/usr/local/bin/check_mailq` exist and are executable. It SHALL NOT require python3, pip3, or system pflogsumm.

#### Scenario: Installer with Go binaries present
- **WHEN** all three Go binaries are installed
- **THEN** the installer proceeds without Python/Perl dependency checks

#### Scenario: Installer with missing Go binaries
- **WHEN** Go binaries are missing
- **THEN** the installer reports which binary is missing and instructs the user to run `make install` from the repo

### Requirement: README documents Go prerequisites
`README_passive.md` SHALL list Go binaries as requirements instead of Python/pflogsumm/pip pygtail, with build+install instructions referencing the repo root `Makefile`.

#### Scenario: README prerequisites section
- **WHEN** a user reads `README_passive.md`
- **THEN** prerequisites include Go binaries at `/usr/local/bin/` and link to build instructions

### Requirement: Zabbix template compatibility
No changes to `template_postfix_passive.xml` are required. All item keys (`postfix[received]`, `postfix.update_data`, `postfix.pfmailq`) SHALL resolve via the updated config and script.

#### Scenario: Template item keys unchanged
- **WHEN** the existing template is attached to a host after migration
- **THEN** all postfix items continue to collect data without template re-import
