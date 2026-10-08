# `update firewall irr asn`

Fetch or refresh IRR prefix\-list for an ASN\.

## Ze command

- Registry path: `update firewall irr asn`
- Usage: `update firewall irr asn <asn>`
- Mode: Daemon
- Wire method: `ze-firewall:update-irr-asn`
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

Queries the IRR server and saves resolved prefixes to the zefs cache\. Creates the cache entry if it does not exist\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `asn` | string | yes | any value of this type | ASN number | The AS number\, 1 to 4294967294\, in plain or dotted form\. The cache key is the decimal spelling with an AS prefix\, so 1\.10 and 65546 name one entry\. |

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
