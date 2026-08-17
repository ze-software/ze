# `show config history`

## Ze command

- Syntax: `show config history`
- Registry path: `show config history`
- Mode: Read-only
- Wire method: `ze-show:config-history`
- Global pipes: yes

List available configuration rollback points. Shows revisions with timestamps and commit metadata. Pair with 'show config diff' to review changes before rolling back.

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
