# `show system goroutines [<mode>]`

## Ze command

- Syntax: `show system goroutines [<mode>]`
- Registry path: `show system goroutines`
- Mode: Read-only
- Wire method: `ze-show:system-goroutines`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Dump goroutine stacks for debugging hangs or deadlocks. Modes: summary (groups by state), blocked (only lock/channel waiters), full (all stacks). Default: summary. Share the output with support when the daemon stops responding.

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
