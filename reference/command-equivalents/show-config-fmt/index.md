# `show config fmt`

Pretty-print the configuration with consistent formatting.

## Ze command

- Registry path: `show config fmt`
- Usage: `show config fmt`
- Mode: Read-only
- Wire method: `ze-show:config-fmt`
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

Normalizes indentation and ordering. Output goes to stdout (read-only). To rewrite the file in place, use 'ze config fmt -w' from the CLI.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Show running, candidate, and diffed configuration

Category: Configuration

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `compare` (verified, vyos-cli)
  - Intent: Show running, candidate, and diffed configuration
- `show configuration` (verified, vyos-cli)
  - Intent: Show running, candidate, and diffed configuration
