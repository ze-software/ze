# `show fleet divergence`

What differs for a managed client that holds\.

## Ze command

- Registry path: `show fleet divergence`
- Usage: `show fleet divergence <name>`
- Mode: Read\-only
- Wire method: `ze-managed:fleet-divergence-show`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: `list`
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, when the answer has rows: match\, count\, first\, last\, display\, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

On a hub\: what differs between the configuration this hub serves a managed client and the one the client holds\, as a list of changes with every secret masked\, their count\, both hashes\, when the client\'s configuration became active and when the hub received it\, and any revert decision\. It answers no configuration text\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Managed client name | The name of a managed client\, as the client entry of a hub server block names it\. The hub keeps one held copy for each client\. |

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
