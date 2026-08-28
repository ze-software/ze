# `show host storage`

## Ze command

- Syntax: `show host storage`
- Registry path: `show host storage`
- Mode: Read-only
- Wire method: `ze-show:host-storage`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show storage devices attached to this box. Returns size, model, transport type (nvme, sata, mmc, virtio), rotational flag, and NVMe firmware version where applicable.

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
