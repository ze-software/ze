# `request interface mac`

Set the MAC address on an interface\.

## Ze command

- Registry path: `request interface mac`
- Usage: `request interface <name> mac <address>`
- Mode: Daemon
- Wire method: `ze-iface:interface-mac`
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

The address is written as xx\:xx\:xx\:xx\:xx\:xx\. Ze checks that form before it calls the backend\, so a malformed address is refused with its own message\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Interface name | The interface that up\, down\, mtu and mac act on\. Each of those commands inherits it\, and migrate names its own interfaces instead\. |
| `address` | string | yes | any value of this type | MAC address\, as xx\:xx\:xx\:xx\:xx\:xx | Six hexadecimal byte pairs separated by colons\, in either case\. The kernel device takes this address in place of the one it has\. |

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
