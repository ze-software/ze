# `show policy routes`

Show policy-based routing rules.

## Ze command

- Registry path: `show policy routes`
- Usage: `show policy routes`
- Mode: Read-only
- Wire method: `ze-show:policy-routes`
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

Lists PBR rules with match criteria and routing actions.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Policy-based routing rules

Category: Firewall

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
