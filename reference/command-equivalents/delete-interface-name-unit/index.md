# `delete interface name unit`

Remove a VLAN sub\-interface\.

## Ze command

- Registry path: `delete interface name unit`
- Usage: `delete interface name <name> unit <vid>`
- Mode: Daemon
- Wire method: `ze-iface:interface-unit-del`
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

The VLAN id is 1 to 4094\. The interface deleted is \<name\>\.\<vid\>\, so the parent interface stays\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Parent interface name | The parent interface\. Ze joins it with the VLAN id into \<name\>\.\<vid\> and deletes that device\. |
| `vid` | uint | yes | any value of this type | VLAN ID | The 802\.1Q tag of the unit to remove\. A tag outside 1 to 4094 is refused before the backend is called\. |

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
