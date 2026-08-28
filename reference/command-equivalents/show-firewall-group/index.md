# `show firewall group`

## Ze command

- Syntax: `show firewall group`
- Registry path: `show firewall group`
- Mode: Read-only
- Wire method: `ze-show:firewall-group`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show members of a firewall address/port group. Without arguments, lists all known groups. With a name, shows the set elements. Reads from the last applied config, not the kernel.

## Mapping intents

### Firewall address, network, and port groups

Category: Firewall

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
