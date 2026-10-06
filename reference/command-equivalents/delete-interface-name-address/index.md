# `delete interface name address`

Remove an IP address from an interface\.

## Ze command

- Registry path: `delete interface name address`
- Usage: `delete interface name <name> address <prefix>`
- Mode: Daemon
- Wire method: `ze-iface:interface-addr-del`
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

Give the address in the same CIDR form it was added with\. The interface stays\, and only that address goes\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Interface name | The interface that holds the address\. Its other addresses stay\. |
| `prefix` | union | yes | any value of this type | Address in CIDR form | An IPv4 or IPv6 address with its prefix length\. Ze passes the string to the netlink backend\, so it must match the string the add used\. |

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
