# `show capture [<count>] [<peer>] [<protocol>] [<tunnel-id>]`

## Ze command

- Syntax: `show capture [<count>] [<peer>] [<protocol>] [<tunnel-id>]`
- Registry path: `show capture`
- Mode: Read-only
- Wire method: `ze-show:capture`
- Global pipes: yes

Show captured control-plane messages. Returns protocol messages you previously enabled capture for. Without a protocol keyword, shows all protocols. Filters: tunnel-id (L2TP), peer (remote address), count (limit entries). Use this to debug session establishment issues.

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
