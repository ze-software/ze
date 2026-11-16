# `update irr asn`

Fetch the IRR dataset of an AS number now\.

## Ze command

- Registry path: `update irr asn`
- Usage: `update irr asn <asn>`
- Mode: Daemon
- Wire method: `ze-irr:update-irr-asn`
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

Asks PeeringDB for the AS\-SET of the AS number\, stores that binding\, then fetches and stores the AS\-SET\'s prefixes\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `asn` | union | yes | any value of this type | AS number\. | The AS number in plain decimal or in asdot notation\, which Ze parses to one 32\-bit value\: 0 to 4294967295 in plain decimal\, 0\.0 to 65535\.65535 in asdot\. A spelling that parses to no AS number is refused\. |

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
