# `show isis database detail`

## Ze command

- Syntax: `show isis database detail`
- Registry path: `show isis database detail`
- Mode: Read-only
- Wire method: `ze-show:isis-database-detail`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show the IS-IS link-state database with TLV detail. Expands each LSP into its decoded TLVs (type, length, value) so you can read exactly what each node advertises. It carries the same fields as the summary view, the own field included.

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
