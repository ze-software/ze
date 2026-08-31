# `show vpn ipsec dataplane drift`

Compare what the IKE engine believes against what the kernel holds.

## Ze command

- Registry path: `show vpn ipsec dataplane drift`
- Usage: `show vpn ipsec dataplane drift`
- Mode: Read-only
- Wire method: `ze-show:vpn-ipsec-dataplane-drift`
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

Reports each Child SA the engine counts as installed whose SPI the kernel SAD does not hold. The command exits non-zero when it finds drift, so a script CAN test it. A rekey window holds two SPIs and is not drift.

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
