# `show system sockets [<port>] [<protocol>] [<state>]`

## Ze command

- Syntax: `show system sockets [<port>] [<protocol>] [<state>]`
- Registry path: `show system sockets`
- Mode: Read-only
- Wire method: `ze-show:system-sockets`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show open TCP and UDP sockets on this box. Filters: [tcp|udp] [state <STATE>] [port <N>], all optional and combinable. States use kernel names (ESTABLISHED, LISTEN, TIME_WAIT). Linux only. Good for confirming listeners or spotting stuck connections.

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
