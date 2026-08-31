# `show firewall group`

Show members of a firewall address/port group.

## Ze command

- Registry path: `show firewall group`
- Usage: `show firewall group`
- Mode: Read-only
- Wire method: `ze-show:firewall-group`
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

Without arguments, lists all known groups. With a name, shows the set elements. Reads from the last applied config, not the kernel.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Firewall address, network, and port groups

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
