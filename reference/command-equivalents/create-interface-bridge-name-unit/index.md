# `create interface bridge name unit`

Add a VLAN sub-interface to the bridge.

## Ze command

- Registry path: `create interface bridge name unit`
- Usage: `create interface bridge name <name> unit <vid>`
- Mode: Daemon
- Wire method: `ze-iface:interface-unit-add`
- Backends: `netlink`
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

The VLAN id is 1 to 4094, and the new interface is named <name>.<vid>. The bridge is created first when it is absent, and deleted again when this step fails.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `name` | string | yes | any value of this type |
| `vid` | uint | yes | any value of this type |

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
