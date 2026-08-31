# `show firewall irr`

Show IRR filter status for all cached ASN/AS-SET entries.

## Ze command

- Registry path: `show firewall irr`
- Usage: `show firewall irr`
- Mode: Read-only
- Wire method: `ze-show:firewall-irr-status`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `prefix`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists each cached entry with prefix counts, last refresh time, and error status. Use this to confirm that IRR prefix-lists are loaded and current before committing firewall config.

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
