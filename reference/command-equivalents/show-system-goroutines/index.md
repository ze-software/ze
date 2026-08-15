# `show system goroutines [<mode>]`

## Ze command

- Syntax: `show system goroutines [<mode>]`
- Registry path: `show system goroutines`
- Mode: Read-only
- Wire method: `ze-show:system-goroutines`
- Global pipes: yes

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
