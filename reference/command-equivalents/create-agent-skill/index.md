# `create agent skill`

Install the ze skill for the operator\'s agent harnesses\.

## Ze command

- Registry path: `create agent skill`
- Usage: `create agent <claude|codex|opencode|pi> skill`
- Mode: daemon\, offline
- Wire method: `ze-skills:create-agent-skill`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, when the answer has rows: match\, count\, first\, last\, display\, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

At a shell\, installs the ze stub skill for every harness found or for the one named\, and with no harness found the shared stub Codex\, OpenCode and Pi read\. For Claude Code and Codex it also adds one session\-start hook to the harness\'s settings file\, leaving every other byte of that file as it was\. Through the daemon\, writes nothing and answers the stub a remote agent places\.

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
