# `show traffic stat`

Show aggregated traffic snapshot \(interface rates\, top talkers\, top ports\, severity\)\.

## Ze command

- Registry path: `show traffic stat`
- Usage: `show traffic stat [name <name>]`
- Mode: Read-only
- Wire method: `ze-show:traffic-stat`
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
| `name` | string | no | any value of this type | Interface name filter | The name of one interface\, matched exactly against the interface names in the snapshot\. A name in no row leaves the answer empty\. |

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
