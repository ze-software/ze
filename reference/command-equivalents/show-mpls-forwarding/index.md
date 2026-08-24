# `show mpls forwarding [<limit>]`

## Ze command

- Syntax: `show mpls forwarding [<limit>]`
- Registry path: `show mpls forwarding`
- Mode: Read-only
- Wire method: `ze-show:mpls-forwarding`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show MPLS forwarding entries installed in the kernel. Each entry shows the incoming label, swap/push/pop operation, and outgoing next-hop. Pass 'limit N' to cap large tables. Linux only.

## Mapping intents

### MPLS, LDP, and RSVP-TE state

Category: Routing protocols

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
