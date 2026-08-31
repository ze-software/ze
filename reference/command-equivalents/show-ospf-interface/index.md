# `show ospf interface`

Show OSPF-enabled interfaces.

## Ze command

- Registry path: `show ospf interface`
- Usage: `show ospf interface`
- Mode: Read-only
- Wire method: `ze-show:ospf-interface`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `detail`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Returns area, network-type, cost, ISM state, DR/BDR, hello/dead intervals, priority, and passive flag per interface.

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
