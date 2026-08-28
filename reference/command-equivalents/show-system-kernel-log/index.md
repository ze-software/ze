# `show system kernel-log [<count>] [<level>]`

## Ze command

- Syntax: `show system kernel-log [<count>] [<level>]`
- Registry path: `show system kernel-log`
- Mode: Read-only
- Wire method: `ze-show:system-kernel-log`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show kernel log messages (dmesg-style). Reads from /dev/kmsg. Filter by syslog level (emerg through debug) and limit with count. Without count, you get everything available. Linux only. Useful for spotting NIC errors or OOM events.

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
