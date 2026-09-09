# `show plugin declarations`

Show what each plugin declares about its command surface.

## Ze command

- Registry path: `show plugin declarations`
- Usage: `show plugin declarations`
- Mode: Read-only
- Wire method: `ze-show:plugin-declarations`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `config`
- Answer shape: tab
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Each row names one plugin, says whether its code is in this binary or in another one, and carries the commands it serves and the pipe aliases it puts on them. Add 'config <path>' and the plugins that file declares gain a row too, external ones included. A plugin whose declaration could not be read keeps its row and says why in the state field, so a plugin is never missing from the answer. 'show plugin list' answers the other question, which plugins this binary carries and what each plugin's own init() recorded.

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
