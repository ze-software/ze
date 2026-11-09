# `show doctor`

Check if this box is ready to run Ze\.

## Ze command

- Registry path: `show doctor`
- Usage: `show doctor [config <config>] [kernel-capabilities]`
- Mode: Read-only
- Wire method: `ze-doctor:show-doctor`
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

Verifies runtime dependencies\: required files\, sockets\, ports\, and kernel modules\, and the checks the daemon\'s plugins declare\. Each check reports pass or fail with a reason\. ze doctor runs the same checks with no daemon\, before first start or after changing the platform setup\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `config` | string | no | any value of this type | Config file to check instead of the active config\. | Check this config file instead of the active config in the store\. On the daemon the file must lie in a config directory of the daemon\'s host\. |
| `kernel-capabilities` | flag | no | any value of this type | Probe every kernel feature Ze enrolls\. | Probe every kernel feature Ze enrolls\, whatever the configuration uses\. The answer is ready only when every feature is present\. |

## Mapping intents

### CPU, memory, platform, and host health

Category: System

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show hardware cpu` (verified, vyos-cli)
  - Intent: CPU, memory, platform, and host health
- `show system memory` (verified, vyos-cli)
  - Intent: CPU, memory, platform, and host health
