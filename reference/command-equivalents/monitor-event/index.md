# `monitor event`

Stream live events as they happen.

## Ze command

- Registry path: `monitor event`
- Usage: `monitor event`
- Mode: Read-only
- Wire method: `ze-event:monitor`
- Backends: any backend
- Task support: required: the MCP server always answers with a task handle
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Shows a real-time feed of internal events. Filter with include <pattern> or exclude <pattern> to focus on what matters. Patterns match event type names.

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
