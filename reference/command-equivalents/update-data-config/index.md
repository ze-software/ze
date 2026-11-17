# `update data config`

Import one config into the daemon\'s store as a new version\.

## Ze command

- Registry path: `update data config`
- Usage: `update data config <name> content <content>`
- Mode: Daemon
- Wire method: `ze-config:data-config-update`
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

Imports one config through the store the daemon owns\, so it works while ze runs\. For the daemon\'s own config\, the content is staged as the candidate and the same reload a SIGHUP runs promotes it\: the config becomes active only when the reload accepts it\, and a refused config leaves the active config\, its pointers and the daemon unchanged\, with the reload\'s reason in the refusal\. The import is refused while a confirmed\-commit window is open\, with the window\'s own refusal\, and while another change is staged\. For any other name\, the content is committed as a new active version\, the previous one kept as rollback\, and nothing reloads\. The answer carries the name\, the version stamp and the byte count\. The content is never recorded\: the audit log\, the accounting record\, the SSH exec log and the session transcript hold a placeholder in its place\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Config file name\, with no directory | The config\'s file name with no directory\, as show config list prints it\. A path\, \. and \.\. are refused\. |
| `content` | string | yes | any value of this type | Config bytes in base64\, at most 195\,840 bytes | The config\'s bytes\, base64\-encoded \(RFC 4648 Section 4\, with padding\)\: 1 to 195\,840 bytes\, at most 261\,120 characters\. The value is never recorded\. |

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
