# `show traffic usage [<name>]`

## Ze command

- Syntax: `show traffic usage [<name>]`
- Registry path: `show traffic usage`
- Mode: Read-only
- Wire method: `ze-show:traffic-usage`
- Global pipes: yes

Show per-interface traffic byte counters captured by eBPF TCX. Per destination/source port and protocol counters are always present; per-IP top-talker counters appear when track-ip is enabled. Without arguments, lists all monitored interfaces. With 'name <interface>', shows that one interface.

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
