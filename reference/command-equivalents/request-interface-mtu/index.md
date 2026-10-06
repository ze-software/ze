# `request interface mtu`

Set the MTU on an interface\.

## Ze command

- Registry path: `request interface mtu`
- Usage: `request interface <name> mtu <bytes>`
- Mode: Daemon
- Wire method: `ze-iface:interface-mtu`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The MTU is between 68 and 65535 bytes\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Interface name | The interface that up\, down\, mtu and mac act on\. Each of those commands inherits it\, and migrate names its own interfaces instead\. |
| `bytes` | uint | yes | any value of this type | MTU in bytes | The largest frame payload the interface accepts\. A value outside 68 to 65535 is refused before the backend is called\. |

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
