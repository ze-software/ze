# `delete agent skill`

Remove the ze skill from the operator\'s agent harnesses\.

## Ze command

- Registry path: `delete agent skill`
- Usage: `delete agent <claude\|codex\|opencode\|pi\|all> skill`
- Mode: daemon, offline
- Wire method: `ze-skills:delete-agent-skill`
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

At a shell\, removes the ze stub skill Ze installed\, for the harness named or\, with no agent or agent all\, for every harness found\, takes its session\-start hook out of the harness\'s settings file\, and leaves every file and hook Ze did not write\. Through the daemon\, writes nothing and answers the file a remote agent removes\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `agent` | enum | no | `claude`, `codex`, `opencode`, `pi`, `all` | Agent harness\, or all\. | The one harness to remove Ze\'s setup from\. Without it\, or with all\, delete acts on every harness whose configuration directory exists\. |

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
