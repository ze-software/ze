# `set debug module`

Enable debug output for one subsystem\.

## Ze command

- Registry path: `set debug module`
- Usage: `set debug module <module-name> [level <level>] [flag <flag>] [scope <kind> <value>]`
- Mode: Daemon
- Wire method: `ze-debug:set-module`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `scope`
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: none
- Pipes, when the answer has rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

The bare command enables the subsystem at its default level\. A level\, a flag or a scope narrows what the subsystem writes\. The profile is saved and applied\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `module-name` | string | yes | any value of this type | Subsystem name\. | The subsystem name\, as show debug lists it\. A name holding a slash is refused\. |
| `level` | string | no | any value of this type | Log level for the subsystem\. | One of disabled\, debug\, info\, warn or error\. Any other word is refused\. |
| `flag` | string | no | any value of this type | Debug flag to enable\. | A flag the subsystem declares\. A flag it does not declare is refused and the valid flags are listed\. |

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
