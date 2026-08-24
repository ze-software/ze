# `show vpn ipsec dataplane sa [<spi>]`

## Ze command

- Syntax: `show vpn ipsec dataplane sa [<spi>]`
- Registry path: `show vpn ipsec dataplane sa`
- Mode: Read-only
- Wire method: `ze-show:vpn-ipsec-dataplane-sa`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show the Security Association Database the kernel holds. Lists each installed ESP SA with its SPI, addresses, mode, algorithms, replay window, byte and packet counters, and timestamps. Give 'spi <spi>' to show one SA. Without a selector the command dumps every SA, which on a device with many tunnels is one row per SA.

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
