# `show bfd session address`

Show full detail for one BFD session.

## Ze command

- Registry path: `show bfd session address`
- Usage: `show bfd session address <address>`
- Mode: Read-only
- Wire method: `ze-bfd-api:show-session`
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

Pass the peer address. Returns local/remote discriminators, negotiated timers, detection time, and packet counters.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `address` | string | yes | any value of this type |

## Mapping intents

### BFD session state

Category: Routing protocols

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
