# `request fleet divergence revert`

Make a client that holds take the hub\'s configuration\.

## Ze command

- Registry path: `request fleet divergence revert`
- Usage: `request fleet divergence revert <name> <version>`
- Mode: Daemon
- Wire method: `ze-managed:fleet-divergence-revert`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, when the answer has rows: match\, count\, first\, last\, display\, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Decides that the client drops the configuration it holds and takes the configuration this hub serves it\, then tells the client\. The decision is bound to the client\'s hash given here and to the hub\'s configuration as it stands now\: a later edit on the client\, or a later write of the client\'s configuration on the hub\, voids it\. A client that is not connected takes it at its next connection\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Managed client name | The name of a managed client\, as the client entry of a hub server block names it\. The hub keeps one held copy for each client\. |
| `version` | string | yes | any value of this type | Device configuration hash | The version hash of the client\'s configuration\, as show fleet divergence name answers it\. A held copy with another hash is refused\, so a decision is made only on the copy the operator saw\. |

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
