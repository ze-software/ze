# `show ospf ipv6 neighbor detail`

Show the full per-neighbor OSPFv3 state (spec-ospf-ext-14).

## Ze command

- Registry path: `show ospf ipv6 neighbor detail`
- Usage: `show ospf ipv6 neighbor detail`
- Mode: Read-only
- Wire method: `ze-show:ospfv3-neighbor-detail`
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

Returns the advertised Interface ID, the DD sequence, the decoded Options (R/V6/E/N/AF), the list sizes, the last NSM event, and the timers.

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
