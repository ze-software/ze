# `show dns lookup`

Look up a DNS name from the router\.

## Ze command

- Registry path: `show dns lookup`
- Usage: `show dns lookup <hostname> [type <A\|AAAA\|MX\|NS\|TXT\|CNAME\|PTR>]`
- Mode: Read-only
- Wire method: `ze-show:dns-lookup`
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

Resolves \<hostname\> using the daemon\'s DNS resolver \(falls back to the system resolver if no DNS component is configured\)\. Default type is A\. Returns records\, TTL\, and query time\. Supports A\, AAAA\, MX\, NS\, TXT\, CNAME\, and PTR\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `hostname` | string | yes | any value of this type | Hostname to resolve\. | A DNS name of at most 253 characters\. The first word that is not the type keyword is taken as the name\, and a call with no name is refused\. |
| `type` | enum | no | `A`, `AAAA`, `MX`, `NS`, `TXT`, `CNAME`, `PTR` | DNS record type\. | The record type to query\, written after the type keyword\. The default is A\. Ze upper\-cases the value\, and a type outside this list is refused with the list of accepted ones\. |

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
