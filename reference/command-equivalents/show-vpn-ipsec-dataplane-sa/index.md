# `show vpn ipsec dataplane sa`

Show the Security Association Database the kernel holds.

## Ze command

- Registry path: `show vpn ipsec dataplane sa`
- Usage: `show vpn ipsec dataplane sa [spi <spi>]`
- Mode: Read-only
- Wire method: `ze-show:vpn-ipsec-dataplane-sa`
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

Lists each installed ESP SA with its SPI, addresses, mode, algorithms, replay window, byte and packet counters, and timestamps. Give 'spi <spi>' to show one SA. Without a selector the command dumps every SA, which on a device with many tunnels is one row per SA.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `spi` | uint | no | any value of this type |

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
