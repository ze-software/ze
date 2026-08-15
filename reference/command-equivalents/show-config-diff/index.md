# `show config diff`

## Ze command

- Syntax: `show config diff`
- Registry path: `show config diff`
- Mode: Read-only
- Wire method: `ze-show:config-diff`
- Global pipes: yes

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
