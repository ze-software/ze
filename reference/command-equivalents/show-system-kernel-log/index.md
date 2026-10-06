# `show system kernel-log`

Show kernel log messages \(dmesg\-style\)\.

## Ze command

- Registry path: `show system kernel-log`
- Usage: `show system kernel-log [level <emerg\|alert\|crit\|err\|warning\|notice\|info\|debug>] [count <count>]`
- Mode: Read-only
- Wire method: `ze-show:system-kernel-log`
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

Reads from \/dev\/kmsg\. Filter by syslog level \(emerg through debug\) and limit with count\. Without count\, you get the newest 50 messages\. Linux only\. Useful for spotting NIC errors or OOM events\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `level` | enum | no | `emerg`, `alert`, `crit`, `err`, `warning`, `notice`, `info`, `debug` | Filter by syslog level\. | The word is the highest syslog level to show\, and every more severe level is included with it\. A digit from 0 to 7 is accepted in its place\. Absent\, or not a known level\, nothing is filtered\. |
| `count` | uint | no | any value of this type | Maximum number of messages\. | The value is from 1 to 10000 and the default is 50\. Ze reads the whole ring buffer\, applies the level filter\, then keeps the newest count messages\. A value outside the range is ignored and the default applies\. |

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
