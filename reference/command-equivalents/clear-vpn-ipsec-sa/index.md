# `clear vpn ipsec sa`

## Ze command

- Syntax: `clear vpn ipsec sa`
- Registry path: `clear vpn ipsec sa`
- Mode: Daemon
- Wire method: `ze-clear:vpn-ipsec-sa`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Tear down IKE Security Associations. Without arguments, terminates all SAs. Use 'peer <name>' to clear just one peer. The tunnel will renegotiate automatically if the config is still active.

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
