# `update fleet divergence adopt`

Make the configuration a client holds the hub\'s configuration for it\.

## Ze command

- Registry path: `update fleet divergence adopt`
- Usage: `update fleet divergence adopt <name> <version>`
- Mode: Daemon
- Wire method: `ze-managed:fleet-divergence-adopt`
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

Makes the configuration the client holds this hub\'s configuration for that client\, as a new version of client\-\<name\>\.conf\, and removes the held copy\. The client already runs it\, so its next fetch is answered current and its hold ends\. The hash given here must be the held copy\'s\.

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
