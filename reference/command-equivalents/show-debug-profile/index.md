# `show debug profile`

Show stored debug profiles\, one by name\, or one filtered to a module subtree\.

## Ze command

- Registry path: `show debug profile`
- Usage: `show debug profile [name <name>] [module <module>]`
- Mode: Read-only
- Wire method: `ze-debug:show-profile`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: none
- Pipes, when the answer has rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

Reads debug\.zefs and asks no daemon\. With no argument it lists the profile names\. name \<name\> prints that profile as a table of module\, level\, flags and scopes\, and module \<prefix\> then keeps the rows under one subsystem subtree\. Any other trailing word is refused rather than ignored\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | no | any value of this type | Profile name\. | The stored profile to print\. Absent\, the command lists the profile names\. |
| `module` | string | no | any value of this type | Subsystem subtree to keep\. | A subsystem prefix\. Only the rows at or under that subtree are printed\, and the keyword is read only after name\. |

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
