# `request data restore`

Restore a configuration now or accept full\-store recovery after shutdown\.

## Ze command

- Registry path: `request data restore`
- Usage: `request data restore <path> [config] [full] [name <name>] [client <client>]`
- Mode: Daemon
- Wire method: `ze-config:data-restore`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
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

Select config to stage one configuration through the normal reload\, or full to accept every artifact key for recovery after a clean daemon shutdown and successful store close\. Full recovery verifies an independent immutable copy before answering\, then requests restart only after response delivery\. A failed shutdown prevents replacement\. With config and client\, a hub updates that served client\'s configuration without reloading its own\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `path` | string | yes | any value of this type | Absolute artifact path | Absolute path of the artifact\, checked before it is read by the same rules as a backup path\. |
| `config` | flag | no | any value of this type | Restore the config only | Restore one selected configuration through the normal reload\. Select exactly one mode\, config or full\. |
| `full` | flag | no | any value of this type | Accept full\-store recovery after a clean shutdown and owner close\. | Accept a verified complete store replacement for the outer daemon lifetime\. It runs only after all runtime borrowers finish\, the original store closes successfully and the daemon returns its existing restart result\. The response confirms acceptance rather than completion\. It cannot be combined with config\, name or client\. |
| `name` | string | no | any value of this type | Config name inside the artifact | The config to take from an artifact that holds several\. Without it\, the artifact\'s only config\, or the config named like this device\, is taken\. |
| `client` | string | no | any value of this type | Managed client whose served config is restored | The managed client whose served config \(client\-\<name\>\.conf\) the restore writes\. The daemon MUST be a hub with a client entry of this name\. Without it\, the restore replaces this daemon\'s own config through the reload\. |

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
