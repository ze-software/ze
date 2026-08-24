# `show config dump`

## Ze command

- Syntax: `show config dump`
- Registry path: `show config dump`
- Mode: Read-only
- Wire method: `ze-show:config-dump`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: none

Show the fully resolved configuration tree. Parses the config and outputs it after includes, defaults, and group inheritance have been applied. What you see here is exactly what the daemon is using.

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
