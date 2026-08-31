# `show audit`

Show who did what and when on this box.

## Ze command

- Registry path: `show audit`
- Usage: `show audit [action <action>] [actor <actor>] [surface <surface>] [since <since>] [until <until>] [count <count>]`
- Mode: Read-only
- Wire method: `ze-show:audit`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Returns audit log entries with timestamps, actors, and actions. Filters (all optional, combinable): action <type>, actor <name>, surface <name> (cli, web, api), since/until <RFC3339>, count <N>. Actions include config-commit, login, peer-teardown, and more.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `action` | string | no | any value of this type |
| `actor` | string | no | any value of this type |
| `surface` | string | no | any value of this type |
| `since` | string | no | any value of this type |
| `until` | string | no | any value of this type |
| `count` | uint | no | any value of this type |

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
