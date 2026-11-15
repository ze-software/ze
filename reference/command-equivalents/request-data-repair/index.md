# `request data repair`

Write a salvage copy of the store to a new path on the daemon\'s host\.

## Ze command

- Registry path: `request data repair`
- Usage: `request data repair <path>`
- Mode: Daemon
- Wire method: `ze-config:data-repair`
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

Writes a salvage copy of the live store to a new store at path\, holding every entry that reads\, while the daemon runs\. The daemon holds its own writes for the walk\, so the copy is one moment of the store\. The history of the copy is then repaired the way ze data repair repairs it\. The answer carries the recovered and skipped counts and the path\. The live store is not changed\: installing the copy needs ze stopped\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `path` | string | yes | any value of this type | Absolute path of the new store | Absolute path of the new store\. A relative path\, a \'\.\.\' element\, a symlink\, a path that exists\, every name the store owns in its folder and a path inside the live tree are refused\, each naming the rule it broke\. |

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
