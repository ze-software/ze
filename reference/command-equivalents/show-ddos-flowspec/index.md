# `show ddos flowspec`

Show the upstream FlowSpec/RTBH DDoS mitigation status: whether a rule is currently announced, the target vector it covers, and whether the leak-probe is running.

## Ze command

- Registry path: `show ddos flowspec`
- Usage: `show ddos flowspec`
- Mode: Read-only
- Wire method: `ze-show:ddos-flowspec`
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
