# `delete debug module`

Disable debug for a subsystem\, or remove one of its flags or scopes\.

## Ze command

- Registry path: `delete debug module`
- Usage: `delete debug module <module-name> [flag <flag>] [scope <kind> <value>]`
- Mode: Daemon
- Wire method: `ze-debug:delete-module`
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

The module name alone removes the whole module\. Naming a flag or a scope removes that one and leaves the module enabled\. Deleting a module the profile does not hold succeeds and changes nothing\, so a repeated command is safe\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `module-name` | string | yes | any value of this type | Subsystem name\. | The subsystem name\, as show debug lists it\. |
| `flag` | string | no | any value of this type | Debug flag to remove\. | The flag to remove\. The module stays enabled\. |

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
