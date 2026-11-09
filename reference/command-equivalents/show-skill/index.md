# `show skill`

Fetch one bundled agent skill by name\.

## Ze command

- Registry path: `show skill`
- Usage: `show skill <name> [full]`
- Mode: read-only, offline
- Wire method: `ze-skills:show-skill`
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

Prints the named skill\'s Markdown document\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Skill name\. | A skill name as show skills prints it\. An unknown name is refused\. |
| `full` | flag | no | any value of this type | Return expanded content \(with examples and patterns\)\. | Print the expanded document\, with examples and patterns\, where the skill carries one\. |

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
