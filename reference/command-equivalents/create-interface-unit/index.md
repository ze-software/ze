# `create interface unit`

Add a VLAN sub\-interface \(802\.1Q tagged\)\.

## Ze command

- Registry path: `create interface unit`
- Usage: `create interface unit <name> <vid>`
- Mode: Daemon
- Wire method: `ze-iface:interface-unit-add`
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

The parent interface must already exist\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Parent interface name | The parent interface\. This form creates no interface\, so the name must be present in the kernel before the command runs\. |
| `vid` | uint | yes | any value of this type | VLAN ID | The 802\.1Q tag of the unit\. The new device is named \<name\>\.\<vid\>\, and a tag outside 1 to 4094 is refused before the backend is called\. |

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
