# `request data config rename`

Rename a stored configuration and preserve all its saved state\.

## Ze command

- Registry path: `request data config rename`
- Usage: `request data config rename <old> <new>`
- Mode: Daemon
- Wire method: `ze-config:data-config-rename`
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

Rename an inactive stored configuration with its dated history\, version pointers\, active and candidate mirrors\, and saved draft\/account changes\. Shared content objects and unrelated audit\/history records are preserved\. Any destination\-owned state or unfinished identity transaction refuses the rename\. The running daemon\'s source and configured managed\-client identities are refused because their launch or fetch bindings cannot be safely rebound by this command\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `old` | string | yes | any value of this type | Existing stored config filename\. | Existing stored configuration name\, without a directory\. |
| `new` | string | yes | any value of this type | Unused new config filename\. | Unused destination configuration name\, without a directory\. No mirror\, pointer\, version or saved editor state may already belong to it\. |

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
