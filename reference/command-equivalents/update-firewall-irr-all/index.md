# `update firewall irr all`

Refresh all cached IRR prefix-lists.

## Ze command

- Registry path: `update firewall irr all`
- Usage: `update firewall irr all`
- Mode: Daemon
- Wire method: `ze-update:firewall-irr-all`
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

Re-queries the IRR server for every cached ASN/AS-SET entry and updates the zefs cache on success. A failed refresh preserves the existing cache and reports an error.

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
