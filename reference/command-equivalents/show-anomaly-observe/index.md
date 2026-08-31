# `show anomaly observe`

Show the behavioral anomaly incident lifecycle, newest first.

## Ze command

- Registry path: `show anomaly observe`
- Usage: `show anomaly observe`
- Mode: Read-only
- Wire method: `ze-show:anomaly-observe`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Per incident: the source entity, cohort, fired features with their deviation z-scores, combined score, severity, start time, end time, and whether it is still active. Finalized incidents stay in the list, so this shows a finished incident's duration, which `show anomaly detect` cannot.

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
