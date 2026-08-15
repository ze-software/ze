# `show system file-descriptors [<mode>]`

## Ze command

- Syntax: `show system file-descriptors [<mode>]`
- Registry path: `show system file-descriptors`
- Mode: Read-only
- Wire method: `ze-show:system-file-descriptors`
- Global pipes: yes

Show how many file descriptors the daemon has open. Summary mode: totals by type (socket, pipe, file). Detail mode: every fd with its path and type. Linux only (reads /proc/self/fd). Check this when you suspect fd exhaustion.

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
