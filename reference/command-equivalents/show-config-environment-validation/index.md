# `show config environment-validation`

Check environment values the answering process cannot use\.

## Ze command

- Registry path: `show config environment-validation`
- Usage: `show config environment-validation`
- Mode: read\-only\, offline
- Wire method: `ze-config-cli:show-config-environment-validation`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Check explicitly set registered environment values against their types and report range failures recorded by their readers\. Only this process\'s environment is judged\; no configuration file or candidate is changed\.

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
