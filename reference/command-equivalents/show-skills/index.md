# `show skills`

List the agent skills this binary carries\.

## Ze command

- Registry path: `show skills`
- Usage: `show skills`
- Mode: Read-only
- Wire method: `ze-skills:show-skills`
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

Each skill is a Markdown document bundled with the binary\, so it always matches the running version\. Prints each bundled skill with its one\-line description\.

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
