# `update data key`

Write one runtime\-state key into the daemon\'s store\.

## Ze command

- Registry path: `update data key`
- Usage: `update data key <key> content <content>`
- Mode: Daemon
- Wire method: `ze-config:data-key-update`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
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

Writes one runtime\-state key \(a key under meta\/ outside meta\/config\/\) through the store the daemon owns\, so it works while ze runs\. The key is created\, or replaced when it exists\, with the bytes content decodes to\, and the answer carries the key and the byte count\. A configuration key \(under file\/ or meta\/config\/\, or outside both\) is refused before the store is touched\, because a raw write bypasses its versions\, pointers and history\. A key a plugin is granted is refused while that plugin runs\, because the plugin manages the key while it runs\; the refusal names the plugin and its restart\. Until the daemon has started its plugins\, such a key is refused\, because the daemon cannot tell yet whether the plugin runs\. The content is never recorded\: the audit log\, the accounting record\, the SSH exec log and the session transcript hold a placeholder in its place\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `key` | string | yes | any value of this type | Store key\, as show data list prints it | The key to write\, as show data list prints it\. |
| `content` | string | yes | any value of this type | Key bytes in base64\, at most 195\,840 bytes | The bytes to write\, base64\-encoded \(RFC 4648 Section 4\, with padding\)\: 1 to 195\,840 bytes\, at most 261\,120 characters\. The value is never recorded\. |

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
