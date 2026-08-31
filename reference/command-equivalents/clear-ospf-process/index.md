# `clear ospf process`

Full OSPF reset: tear down every adjacency and re-run SPF.

## Ze command

- Registry path: `clear ospf process`
- Usage: `clear ospf process`
- Mode: Daemon
- Wire method: `ze-clear:ospf-process`
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

Adjacencies re-form from the next Hello; the configuration is unchanged.

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
