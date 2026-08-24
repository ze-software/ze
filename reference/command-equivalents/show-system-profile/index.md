# `show system profile [<duration>] [<type>]`

## Ze command

- Syntax: `show system profile [<duration>] [<type>]`
- Registry path: `show system profile`
- Mode: Read-only
- Wire method: `ze-show:system-profile`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Capture a runtime profile for performance analysis. Types: cpu (requires duration, e.g. 30s), heap, goroutine, allocs (instant snapshots). Output is pprof format you can open with 'go tool pprof'. Send the file to support for deep analysis.

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
