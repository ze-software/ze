# `show anomaly observe`

## Ze command

- Syntax: `show anomaly observe`
- Registry path: `show anomaly observe`
- Mode: Read-only
- Wire method: `ze-show:anomaly-observe`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show the behavioral anomaly incident lifecycle, newest first: per incident the source entity, cohort, fired features with their deviation z-scores, combined score, severity, start time, end time, and whether it is still active. Finalized incidents stay in the list, so this shows a finished incident's duration, which `show anomaly detect` cannot.

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
