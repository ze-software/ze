# `show subscriber <id> detail`

## Ze command

- Syntax: `show subscriber <id> detail`
- Registry path: `show subscriber id detail`
- Mode: Read-only
- Wire method: `ze-subscriber-api:detail`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show everything about one subscriber session. Pass the session ID. Returns access type, assigned addresses, authentication state, uptime, and traffic counters.

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
