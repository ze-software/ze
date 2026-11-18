# `show ospf database router-information`

Show the Router Information LSAs \(RFC 7770\) for both address families\.

## Ze command

- Registry path: `show ospf database router-information`
- Usage: `show ospf database router-information [address-family <ipv6-unicast|ipv6-multicast|ipv4-unicast|ipv4-multicast>]`
- Mode: Read\-only
- Wire method: `ze-ospf:show-database-router-information`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Column order: af\, address\-family\, instance\-id
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

OSPFv2 opaque type 4 and OSPFv3 function code 12\, decoded into the advertised informational capability bits and the TLV list\. The address\-family keyword narrows the OSPFv3 part to one address family and leaves the OSPFv2 part whole\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `address-family` | enum | no | `ipv6-unicast`, `ipv6-multicast`, `ipv4-unicast`, `ipv4-multicast` | Limit the answer to one OSPFv3 address family\. | Without it the view answers for every running OSPFv3 address family\, and each row names its address\-family and instance\-id\. With it the view answers for that one family\. A family that is not running is refused\, and the refusal names the running ones\; show ospf ipv6 instance lists them too\. |

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
