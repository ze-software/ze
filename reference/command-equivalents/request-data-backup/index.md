# `request data backup`

Write a backup of the whole store to a file on the daemon\'s host\.

## Ze command

- Registry path: `request data backup`
- Usage: `request data backup <path> [spare <spare>] [force]`
- Mode: Daemon
- Wire method: `ze-data:backup`
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

Copies every key of the store into one blob artifact while the daemon runs\. A commit that lands meanwhile is in the backup whole or not at all\. The file holds credentials and private keys and is written 0600\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `path` | string | yes | any value of this type | Absolute artifact path | Absolute path of the artifact\. A relative path\, a \'\.\.\' element\, a symlink\, and every name the store owns in its folder are refused\, each naming the rule it broke\. |
| `spare` | uint | no | any value of this type | Spare capacity percent \(0\-100\) | Spare capacity in percent that each key and data slot carries\. An artifact is normally copied whole\, so the default is 0\. |
| `force` | flag | no | any value of this type | Replace an existing file | Replace an existing file at path\. It never lifts a refusal of a name the store owns\. |

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
