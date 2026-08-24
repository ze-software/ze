# `show capture interface [<count>] [<duration>] [<format>] [<iface>] [<protocol>] [<snap-len>]`

## Ze command

- Syntax: `show capture interface [<count>] [<duration>] [<format>] [<iface>] [<protocol>] [<snap-len>]`
- Registry path: `show capture interface`
- Mode: Read-only
- Wire method: `ze-show:capture-interface`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Capture live packets on an interface (like tcpdump). Uses AF_PACKET for zero-copy capture. Filter by protocol and port. Limit with count or duration. Output as pcap (for Wireshark) or text. Snap-len controls how many bytes per packet are captured.

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
