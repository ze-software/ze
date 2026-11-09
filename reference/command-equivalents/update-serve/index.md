# `update serve`

Show the update server\'s status\, or start\, stop and pause it\.

## Ze command

- Registry path: `update serve`
- Usage: `update serve`
- Mode: daemon, offline
- Wire method: `ze-update:update-serve`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `pause`, `start`, `stop`
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The daemon serves its running binary and a version manifest for update checks\, until it is told to stop\. A daemon runs one such server at a time\. Through the daemon\, \'update serve\' alone answers that server\'s status\: whether it runs\, the address it listens on and whether it is paused\. The foreground \'ze update serve\'\, given the address to listen on\, runs the same server in the operator\'s own process instead\.

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
