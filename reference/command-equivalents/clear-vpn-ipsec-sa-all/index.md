# `clear vpn ipsec sa all`

Tear down every IKE Security Association\.

## Ze command

- Registry path: `clear vpn ipsec sa all`
- Usage: `clear vpn ipsec sa all`
- Mode: Daemon
- Wire method: `ze-ike:clear-vpn-ipsec-sa`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Every IKE SA and its Child SAs are terminated\, and the answer counts them\. Each tunnel whose config is still active WILL renegotiate\.

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
