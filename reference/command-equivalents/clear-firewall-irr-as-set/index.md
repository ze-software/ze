# `clear firewall irr as-set <as-set>`

## Ze command

- Syntax: `clear firewall irr as-set <as-set>`
- Registry path: `clear firewall irr as-set`
- Mode: Daemon
- Wire method: `ze-clear:firewall-irr-as-set`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Remove the cached IRR prefix-list for an AS-SET. Usage: clear firewall irr as-set <as-set>. Drops the entry from memory and from the persisted cache, then re-applies the firewall tables.

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
