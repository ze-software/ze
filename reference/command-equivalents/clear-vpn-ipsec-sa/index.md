# `clear vpn ipsec sa`

Tear down IKE Security Associations.

## Ze command

- Registry path: `clear vpn ipsec sa`
- Usage: `clear vpn ipsec sa`
- Mode: Daemon
- Wire method: `ze-clear:vpn-ipsec-sa`
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

Without arguments, terminates all SAs. Use 'peer <name>' to clear just one peer. The tunnel WILL renegotiate automatically if the config is still active.

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
