# `create bgp peer`

Add a peer to the running daemon.

## Ze command

- Registry path: `create bgp peer`
- Usage: `create bgp peer <selector> <asn> [local-as <local-as>] [local-address <local-address>] [router-id <router-id>] [receive-hold-time <receive-hold-time>] [send-hold-time <send-hold-time>] [connect-retry <connect-retry>] [connect <true\|false>] [accept <true\|false>] [family <family>] [graceful-restart <graceful-restart>] [group-updates <true\|false>] [attach <attach>]`
- Mode: Daemon
- Wire method: `ze-bgp:peer-add`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The peer address and 'asn' are required. Every other keyword is optional. Ze builds the peer, starts it, and dials the address unless 'connect false' says otherwise. The peer lives in the running daemon alone. Ze writes nothing to the configuration, so a reload removes the peer. 'delete bgp peer' does the same on the way out, and it does not change the file on disk. An address that already names a peer is refused, and so is a keyword Ze cannot honor. Each name in 'attach' is bound to the whole surface: that process receives every message from this peer and can send every type toward it. Write the receive and send lists in the configuration file when the process needs less.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | union | yes | any value of this type |
| `asn` | union | yes | any value of this type |
| `local-as` | union | no | any value of this type |
| `local-address` | union | no | any value of this type |
| `router-id` | string | no | any value of this type |
| `receive-hold-time` | uint | no | any value of this type |
| `send-hold-time` | uint | no | any value of this type |
| `connect-retry` | uint | no | any value of this type |
| `connect` | enum | no | `true`, `false` |
| `accept` | enum | no | `true`, `false` |
| `family` | string | no | any value of this type |
| `graceful-restart` | uint | no | any value of this type |
| `group-updates` | enum | no | `true`, `false` |
| `attach` | string | no | any value of this type |

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
