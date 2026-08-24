# `request commit`

## Ze command

- Syntax: `request commit`
- Registry path: `request commit`
- Mode: Daemon
- Wire method: `ze-bgp:commit`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Group route changes into named atomic commits. Actions: start (begin a commit), end (finalize), eor (signal end of RIB), rollback (undo), show (inspect), withdraw (remove all routes in a commit), list (show all commits). Grammar: request commit <action> <name> [args].

## Mapping intents

### Validate and commit configuration

Category: Configuration

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `commit` (verified, vyos-cli)
  - Intent: Validate and commit configuration
