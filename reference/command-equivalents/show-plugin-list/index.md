# `show plugin list`

List the plugins compiled into this binary.

## Ze command

- Registry path: `show plugin list`
- Usage: `show plugin list`
- Mode: Read-only
- Wire method: `ze-show:plugin-list`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Each row carries the plugin name, what it does, the address families it handles, the RFCs it implements, the BGP capability codes it owns, the setup outcome its own init() recorded (succeeded, soft-failure, hard-failure or unknown) and the reason the plugin gave. A plugin that recorded nothing is listed as unknown, never omitted, and a plugin that recorded and then did not register keeps its row. The outcome replays a past event: it was recorded once, before main(), and this command reads it back. 'show health' answers the other question, by running a probe now and reporting what the daemon is doing at this moment.

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
