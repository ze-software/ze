# `show vpp trace start`

Start capturing packets in the VPP dataplane.

## Ze command

- Registry path: `show vpp trace start`
- Usage: `show vpp trace start`
- Mode: Read-only
- Wire method: `ze-show:vpp-trace-start`
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

Default input node is dpdk-input, default count is 100 (max 10000). After starting, use 'show vpp trace show' to retrieve the captured packets. Requires the VPP backend.

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
