# `show mpls forwarding`

Show MPLS forwarding entries installed in the kernel.

## Ze command

- Registry path: `show mpls forwarding`
- Usage: `show mpls forwarding [limit <limit>]`
- Mode: Read-only
- Wire method: `ze-show:mpls-forwarding`
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

Each entry shows the incoming label, swap/push/pop operation, and outgoing next-hop. Pass 'limit N' to cap large tables. Linux only.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `limit` | uint | no | any value of this type |

## Mapping intents

### MPLS, LDP, and RSVP-TE state

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
