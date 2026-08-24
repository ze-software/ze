# `show vpn ipsec dataplane drift`

## Ze command

- Syntax: `show vpn ipsec dataplane drift`
- Registry path: `show vpn ipsec dataplane drift`
- Mode: Read-only
- Wire method: `ze-show:vpn-ipsec-dataplane-drift`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Compare what the IKE engine believes against what the kernel holds. Reports each Child SA the engine counts as installed whose SPI the kernel SAD does not hold. The command exits non-zero when it finds drift, so a script CAN test it. A rekey window holds two SPIs and is not drift.

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
