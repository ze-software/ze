# `monitor traffic stat`

Start streaming traffic monitor \(per\-second snapshots\)\.

## Ze command

- Registry path: `monitor traffic stat`
- Usage: `monitor traffic stat [name <name>]`
- Mode: Read-only
- Wire method: `ze-trafficstat:monitor-traffic-stat`
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

Without arguments\, shows all interfaces\. With \'name \<interface\>\'\, filters to one interface\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | no | any value of this type | Interface name filter | The name of one interface\, matched exactly against the interface names in each snapshot\. Every other interface is left out of the stream\. |

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
