# `show audit`

Show who did what and when on this box\.

## Ze command

- Registry path: `show audit`
- Usage: `show audit [action <action>] [actor <actor>] [surface <surface>] [since <since>] [until <until>] [count <count>]`
- Mode: Read-only
- Wire method: `ze-cmd:show-audit`
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

Returns audit log entries with timestamps\, actors\, and actions\. Filters \(all optional\, combinable\)\: action \<type\>\, actor \<name\>\, surface \<name\> \(cli\, web\, api\)\, since\/until \<RFC3339\>\, count \<N\>\. Actions include config\-commit\, login\, peer\-teardown\, and more\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `action` | string | no | any value of this type | Filter by action type | The value is an action name as the action column reports it\, such as config\-commit or login\, compared as text\. Absent\, entries of every action are shown\. |
| `actor` | string | no | any value of this type | Filter by actor name | The value is the name of the user or process that acted\, compared as text with the actor column\. Absent\, entries of every actor are shown\. |
| `surface` | string | no | any value of this type | Filter by surface name | The value is the surface the action came through\: cli\, web or api\, compared as text\. Absent\, entries of every surface are shown\. |
| `since` | string | no | any value of this type | Start time \(RFC3339\) | The value is an RFC 3339 timestamp such as 2026\-09\-15T08\:00\:00Z\, and any other form is refused\. Entries recorded before it are left out\. |
| `until` | string | no | any value of this type | End time \(RFC3339\) | The value is an RFC 3339 timestamp such as 2026\-09\-15T18\:00\:00Z\, and any other form is refused\. Entries recorded after it are left out\. |
| `count` | uint | no | any value of this type | Maximum number of entries | The value is a whole number of 1 or more\. The query stops after that many matching entries\, oldest first\. Absent\, every matching entry is shown\. |

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
