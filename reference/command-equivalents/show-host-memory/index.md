# `show host memory`

## Ze command

- Syntax: `show host memory`
- Registry path: `show host memory`
- Mode: Read-only
- Wire method: `ze-show:host-memory`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show installed memory and ECC health. Returns DIMM sizes and, when the edac driver is present, correctable and uncorrectable error counters. Non-zero ECC counts mean you should plan a DIMM replacement.

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
