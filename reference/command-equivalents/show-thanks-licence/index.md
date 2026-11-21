# `show thanks licence`

Print the licence texts of the third\-party works this binary carries\.

## Ze command

- Registry path: `show thanks licence`
- Usage: `show thanks licence`
- Mode: Read\-only
- Wire method: `ze-thanks:show-licence`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: blob
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: `name <value>`: Print the licence of the one work with this exact name\.
- Pipe aliases: none

Prints one document\: for each distinct licence text\, the works it covers with their copyright lines and source location\, then the text as the works ship it\. The name pipe operation narrows it to one work\.

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
