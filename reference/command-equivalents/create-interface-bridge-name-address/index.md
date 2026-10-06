# `create interface bridge name address`

Add an IP address to the bridge\.

## Ze command

- Registry path: `create interface bridge name address`
- Usage: `create interface bridge name <name> address <prefix>`
- Mode: Daemon
- Wire method: `ze-iface:interface-addr-add`
- Backends: `netlink`
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

The address is written in CIDR form\, for example 10\.0\.0\.1\/32\. The bridge is created first when it is absent\, and deleted again when this step fails\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Interface name | The bridge that receives the address\. The dispatcher creates it first when it is absent\. |
| `prefix` | union | yes | any value of this type | Address in CIDR form | An IPv4 or IPv6 address with its prefix length\, for example 10\.0\.0\.1\/32\. Ze passes the string to the netlink backend\. |

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
