# `resolve dns ptr`

Reverse-lookup an IP address to its hostname (PTR).

## Ze command

- Registry path: `resolve dns ptr`
- Usage: `resolve dns ptr <ip-address>`
- Mode: Read-only
- Wire method: `ze-resolve:dns-ptr`
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

The address is turned into its in-addr.arpa or ip6.arpa name before the query. The cache holds the answer under that name, so show dns cache record takes it in that form.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `ip-address` | string | yes | any value of this type |

## Mapping intents

### DNS lookup and cache inspection

Category: Diagnostics

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show dns` (verified, vyos-cli)
  - Intent: DNS lookup and cache inspection
