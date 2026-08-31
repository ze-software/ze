# `show host nic`

Show physical NICs installed in this box.

## Ze command

- Registry path: `show host nic`
- Usage: `show host nic`
- Mode: Read-only
- Wire method: `ze-show:host-nic`
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

Returns driver, PCI vendor/device IDs, link speed, queue counts, and firmware version. Virtual interfaces are excluded. Use this to confirm NIC firmware before an upgrade.

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
