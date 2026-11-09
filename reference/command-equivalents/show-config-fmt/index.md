# `show config fmt`

Pretty\-print the configuration with consistent formatting\.

## Ze command

- Registry path: `show config fmt`
- Usage: `show config fmt [check] [diff]`
- Mode: read-only, offline
- Wire method: `ze-config-cli:show-config-fmt`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: blob
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Normalizes indentation and ordering\. Output goes to stdout \(read\-only\)\. To rewrite the file in place\, use \'ze config fmt \-w\' from the CLI\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `check` | flag | no | any value of this type | Report whether formatting is needed | Answers whether the file needs formatting instead of the formatted text\: exit 0 when it is formatted\, 1 with the file named when it is not\. |
| `diff` | flag | no | any value of this type | Show what formatting would change | Answers the unified diff formatting would apply instead of the formatted text\. |

## Mapping intents

### Show running, candidate, and diffed configuration

Category: Configuration

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `compare` (verified, vyos-cli)
  - Intent: Show running, candidate, and diffed configuration
- `show configuration` (verified, vyos-cli)
  - Intent: Show running, candidate, and diffed configuration
