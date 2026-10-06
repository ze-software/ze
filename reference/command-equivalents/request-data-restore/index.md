# `request data restore`

Restore a config from a backup file on the daemon\'s host\.

## Ze command

- Registry path: `request data restore`
- Usage: `request data restore <path> [config] [name <name>] [client <client>]`
- Mode: Daemon
- Wire method: `ze-data:restore`
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

Stages the artifact\'s config as the candidate and runs the same reload a SIGHUP runs\. The config becomes active only when the reload accepts it\; a refused config leaves the active config and its pointers untouched\. Nothing else in the store changes\. With client\, a hub writes the config it serves to that managed client instead\, as a new version\, and pushes config\-changed to the client\; the hub does not reload its own config\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `path` | string | yes | any value of this type | Absolute artifact path | Absolute path of the artifact\, checked before it is read by the same rules as a backup path\. |
| `config` | flag | no | any value of this type | Restore the config only | Restore the artifact\'s config only\. A full restore replaces the whole store and runs offline with ze data restore\. |
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
