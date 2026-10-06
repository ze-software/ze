# `resolve peeringdb max-prefix`

Get max\-prefix limits for an ASN from PeeringDB\.

## Ze command

- Registry path: `resolve peeringdb max-prefix`
- Usage: `resolve peeringdb max-prefix <asn>`
- Mode: Read-only
- Wire method: `ze-resolve:peeringdb-max-prefix`
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

Returns IPv4 and IPv6 prefix limits\. Apply via the config editor\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `asn` | union | yes | any value of this type | AS number\. | The AS number in plain decimal or in asdot notation\, which Ze parses to one 32\-bit value before the query\. A spelling that parses to no AS number is refused\. |

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
