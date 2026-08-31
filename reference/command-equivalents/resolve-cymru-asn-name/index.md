# `resolve cymru asn-name`

Find out who owns an AS number.

## Ze command

- Registry path: `resolve cymru asn-name`
- Usage: `resolve cymru asn-name <asn>`
- Mode: Read-only
- Wire method: `ze-resolve:cymru-asn-name`
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

Queries Team Cymru DNS to return the organization name for the ASN.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `asn` | uint | yes | any value of this type |

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
