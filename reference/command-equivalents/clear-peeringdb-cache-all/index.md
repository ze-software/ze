# `clear peeringdb cache all`

Remove every cached PeeringDB answer\.

## Ze command

- Registry path: `clear peeringdb cache all`
- Usage: `clear peeringdb cache all`
- Mode: Daemon
- Wire method: `ze-resolve:clear-peeringdb-cache-all`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The answer gives removed\, the entries removed in total\, and owners\, one row per owner with the entries it removed\. The next lookup of each AS number queries PeeringDB again\. An owner that does not answer makes the command an error naming it\.

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
