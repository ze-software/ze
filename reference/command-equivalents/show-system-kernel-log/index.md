# `show system kernel-log`

Show kernel log messages (dmesg-style).

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
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Reads from /dev/kmsg. Filter by syslog level (emerg through debug) and limit with count. Without count, you get everything available. Linux only. Useful for spotting NIC errors or OOM events.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `level` | enum | no | `emerg`, `alert`, `crit`, `err`, `warning`, `notice`, `info`, `debug` |
| `count` | uint | no | any value of this type |

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
