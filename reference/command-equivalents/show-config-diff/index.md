# `show config diff`

## Ze command

- Syntax: `show config diff`
- Registry path: `show config diff`
- Mode: Read-only
- Wire method: `ze-show:config-diff`
- Answer shape: not declared
- Address fields: none
- Pipes, always: none
- Pipes, on rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

Compare two configuration versions side by side. Shows what was added, removed, or changed. Commonly used with rollback revisions to review what changed before you roll back.

## Mapping intents

### Show running, candidate, and diffed configuration

Category: Configuration

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `compare` (verified, vyos-cli)
  - Intent: Show running, candidate, and diffed configuration
- `show configuration` (verified, vyos-cli)
  - Intent: Show running, candidate, and diffed configuration
