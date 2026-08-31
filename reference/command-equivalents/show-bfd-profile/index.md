# `show bfd profile`

Show BFD timer profiles with effective values.

## Ze command

- Registry path: `show bfd profile`
- Usage: `show bfd profile`
- Mode: Read-only
- Wire method: `ze-bfd-api:show-profile`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `name`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Returns min-tx, min-rx, and detect-multiplier after inheritance. Use 'show bfd profile' for every profile or 'show bfd profile name <name>' for one profile.

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
