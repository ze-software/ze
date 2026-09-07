# `show anomaly detect`

Show the recent behavioral anomaly incidents.

## Ze command

- Registry path: `show anomaly detect`
- Usage: `show anomaly detect`
- Mode: Read-only
- Wire method: `ze-show:anomaly`
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

Each incident carries the source entity, the cohort, the fired features with their deviation z-scores, the combined score, and the severity. The detector only reports. The anomaly shape responder (Spec 2b) acts on the report.

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
