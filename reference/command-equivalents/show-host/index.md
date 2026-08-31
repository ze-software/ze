# `show host`

Hardware inventory for this box.

## Ze command

- Registry path: `show host`
- Usage: `show host`
- Mode: Read-only
- Wire method: `ze-show:host-all`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `all`, `cpu`, `dmi`, `kernel`, `memory`, `nic`, `platform`, `storage`, `thermal`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Sections: cpu, nic, dmi, memory, thermal, storage, kernel, platform. Use a subcommand for one section, or 'show host all' for everything. The bare 'show host' is an alias of 'show host all'.

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
