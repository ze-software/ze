# `show config fmt`

## Ze command

- Syntax: `show config fmt`
- Registry path: `show config fmt`
- Mode: Read-only
- Wire method: `ze-show:config-fmt`
- Answer shape: not declared
- Address fields: none
- Pipes, always: none
- Pipes, on rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

Pretty-print the configuration with consistent formatting. Normalizes indentation and ordering. Output goes to stdout (read-only). To rewrite the file in place, use 'ze config fmt -w' from the CLI.

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
