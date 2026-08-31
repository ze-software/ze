# `show reload-status`

Show how many config reloads the daemon has processed.

## Ze command

- Registry path: `show reload-status`
- Usage: `show reload-status`
- Mode: Read-only
- Wire method: `ze-show:reload-status`
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

Returns a generation counter, the outcome of the most recent reload (applied or failed), and when it finished. The counter advances on every processed reload, including one that rejected or changed nothing, so you can confirm a SIGHUP was acted on even when it deliberately left the running config alone.

## Arguments

No command-specific arguments listed.

## Mapping intents

No vendor equivalent has been curated yet for this Ze command.

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
