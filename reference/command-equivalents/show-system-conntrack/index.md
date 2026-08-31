# `show system conntrack`

Show the kernel connection tracking table.

## Ze command

- Registry path: `show system conntrack`
- Usage: `show system conntrack`
- Mode: Read-only
- Wire method: `ze-show:system-conntrack`
- Backends: `nft`
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

Returns conntrack entry count, table size, timeouts, and loaded modules. Requires the nft backend. Check this when you suspect conntrack table exhaustion is dropping traffic.

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
