# `clear l2tp tunnel all`

## Ze command

- Syntax: `clear l2tp tunnel all`
- Registry path: `clear l2tp tunnel all`
- Mode: Daemon
- Wire method: `ze-l2tp-api:tunnel-teardown-all`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Tear down every L2TP tunnel on this box. Sends StopCCN for all tunnels. Every subscriber session will be disconnected. Use with care during maintenance.

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
