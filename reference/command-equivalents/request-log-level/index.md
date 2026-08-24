# `request log level <logger> <level>`

## Ze command

- Syntax: `request log level <logger> <level>`
- Registry path: `request log level`
- Mode: Daemon
- Wire method: `ze-bgp:log-set`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Change a subsystem's log level without restarting. Usage: request log level <logger> <level>. Takes effect immediately. Set to debug when troubleshooting, then back to info when you are done.

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
