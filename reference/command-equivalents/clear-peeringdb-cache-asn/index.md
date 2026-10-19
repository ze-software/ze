# `clear peeringdb cache asn`

Remove the cached PeeringDB answers of one AS number\.

## Ze command

- Registry path: `clear peeringdb cache asn`
- Usage: `clear peeringdb cache asn <as-number>`
- Mode: Daemon
- Wire method: `ze-resolve:clear-peeringdb-cache-asn`
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

Removes the prefix counts and the AS\-SET names of one AS number at every owner and keeps the other entries\. The answer reports found true when any owner held one\, removed\, and the owners rows\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `as-number` | union | yes | any value of this type | AS number to clear\. | The AS number in plain decimal or in asdot notation\, which Ze parses to one 32\-bit value\. A spelling that parses to no AS number is refused\. |

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
