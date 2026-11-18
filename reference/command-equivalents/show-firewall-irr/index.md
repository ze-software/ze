# `show firewall irr`

Show the IRR lists the firewall enforces\.

## Ze command

- Registry path: `show firewall irr`
- Usage: `show firewall irr`
- Mode: Read\-only
- Wire method: `ze-firewall:show-irr-status`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: `prefix`
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, when the answer has rows: match\, count\, first\, last\, display\, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists each configured reference with the stored copy the firewall enforces for it\: its status\, prefix counts\, server\, generation\, check and refresh times\. Use this to confirm that IRR prefix\-lists are stored and current before committing firewall config\.

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
