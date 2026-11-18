# `request plugin restart`

Restart a plugin\, stopping it first if it runs\.

## Ze command

- Registry path: `request plugin restart`
- Usage: `request plugin restart <name>`
- Mode: Daemon
- Wire method: `ze-plugin:system-plugin-restart`
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

Starts a configured plugin again\. A stopped one starts\: one its failure policy left stopped\, one the respawn limit disabled \(the order lifts the limit\)\, one whose restart or startup handshake failed\. A running one is stopped first\, which cuts the commands it was serving\, and started again with the running configuration\; its failure policy is not applied\. ze doctor names this command as the fix of doctor\-plugin\-stopped and of the rows a new process clears\. A name no configuration or auto\-load started is refused\, and so is a restart a plugin asks for itself \(its own request would never end\)\, a restart while a config transaction is in flight\, and a restart of a plugin whose removal by a reload is still pending\. A restart whose old process has not finished its cleanup fails and leaves the plugin down until that cleanup ends\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Name of the configured plugin to start again\. | The plugin\'s configured name\, as ze doctor and show health name it\. |

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
