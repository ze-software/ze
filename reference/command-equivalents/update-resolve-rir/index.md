# `update resolve rir`

Refresh the RIR delegation table from the five registry delegation files.

## Ze command

- Registry path: `update resolve rir`
- Usage: `update resolve rir`
- Mode: Daemon
- Wire method: `ze-update:resolve-rir`
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

Fetches the delegation file of each of the five Regional Internet Registries, parses them, and stores one table under the meta/rir/delegation key. All-or-nothing: a run in which any fetch or parse fails stores nothing and leaves the previous copy in place.

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
