# `request commit`

Group route changes into named atomic commits.

## Ze command

- Registry path: `request commit`
- Usage: `request commit`
- Mode: Daemon
- Wire method: `ze-bgp:commit`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Actions: start (begin a commit), end (finalize), eor (signal end of RIB), rollback (undo), show (inspect), withdraw (remove all routes in a commit), list (show all commits). Grammar: request commit <action> <name> [args].

## Arguments

No command-specific arguments listed.

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
