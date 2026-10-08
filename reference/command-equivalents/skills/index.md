# `skills`

List the agent skills this binary carries\, or fetch one by name\.

## Ze command

- Registry path: `skills`
- Usage: `skills (list\|get <name>)`
- Mode: Daemon
- Wire method: `ze-skills:skills`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `action`
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: none
- Pipes, when the answer has rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

Each skill is a Markdown document bundled with the binary\, so it always matches the running version\. list names them\, and get prints one by name\.

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
