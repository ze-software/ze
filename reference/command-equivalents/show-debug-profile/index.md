# `show debug profile`

Show stored debug profiles, one by name, or one filtered to a module subtree.

## Ze command

- Registry path: `show debug profile`
- Mode: Offline
- Wire method: `not listed`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: none
- Pipes, when the answer has rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

With no argument it lists the profile names. `name <name>` prints that profile as a table of module, level, flags and scopes. Adding `module <prefix>` keeps the rows under one subsystem subtree. Any other trailing word is refused rather than ignored.

## Arguments

No command-specific arguments listed.

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
