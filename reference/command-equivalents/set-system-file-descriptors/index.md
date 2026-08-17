# `set system file-descriptors [<limit>]`

## Ze command

- Syntax: `set system file-descriptors [<limit>]`
- Registry path: `set system file-descriptors`
- Mode: Daemon
- Wire method: `ze-set:system-file-descriptors`
- Global pipes: yes

Raise the file descriptor limit for the daemon process. Pass a number or 'max' to go to the hard limit. Takes effect immediately. Check current limits with 'show system file-descriptors'.

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
