# `delete data key`

Remove one runtime\-state key from the daemon\'s store\.

## Ze command

- Registry path: `delete data key`
- Usage: `delete data key <key>`
- Mode: Daemon
- Wire method: `ze-config:data-key-delete`
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

Removes one runtime\-state key \(a key under meta\/ outside meta\/config\/\) through the store the daemon owns\, so it works while ze runs\. A configuration key \(under file\/ or meta\/config\/\, or outside both\) is refused before the store is touched\, because a raw removal bypasses its versions\, pointers and history\. A key a plugin is granted is refused while that plugin runs\, because the plugin manages the key while it runs\; the refusal names the plugin and its restart\. Until the daemon has started its plugins\, such a key is refused\, because the daemon cannot tell yet whether the plugin runs\. A key the store does not hold is an error\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `key` | string | yes | any value of this type | Store key\, as show data list prints it | The key to remove\, as show data list prints it\. |

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
