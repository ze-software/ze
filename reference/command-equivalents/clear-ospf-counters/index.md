# `clear ospf counters`

Reset the OSPF SPF-run history.

## Ze command

- Registry path: `clear ospf counters`
- Usage: `clear ospf counters`
- Mode: Daemon
- Wire method: `ze-clear:ospf-counters`
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

Monotonic Prometheus series are not reset; the SPF-run log is cleared.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Clear OSPF process, counters, or neighbor state

Category: Routing protocols

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
