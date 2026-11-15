# `show agent skill`

Show whether the ze skill is set up for each agent harness\.

## Ze command

- Registry path: `show agent skill`
- Usage: `show agent <claude\|codex\|opencode\|pi> skill`
- Mode: read-only, offline
- Wire method: `ze-skills:show-agent-skill`
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

At a shell\, reports for each harness the plugin knows whether it was found and whether the ze stub skill and\, for Claude Code and Codex\, the session\-start hook are installed\, and changes no file\. Through the daemon\, answers the stub a remote agent places\, since the daemon cannot see the caller\'s home\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `agent` | enum | no | `claude`, `codex`, `opencode`, `pi` | Agent harness\. | The one harness to act on\. Without it create acts on every harness whose configuration directory exists\, and show reports every harness Ze knows\. |

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
