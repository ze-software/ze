# `clear bgp rib out-excluding-owner`

Replay while excluding one original destination owner\.

## Ze command

- Registry path: `clear bgp rib out-excluding-owner`
- Usage: `clear bgp rib out-excluding-owner`
- Mode: Daemon
- Wire method: `ze-bgp:rib-clear-out-excluding-owner`
- Backends: any backend
- Task support: forbidden\: the MCP server never answers with a task handle
- Subcommands: none: this command takes no subcommand
- Answer shape: map
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Re\-advertise to current destinations except one captured original peer owner\. This narrows replay and grants no send permission\. The ordinary out command and its literal peer selectors keep their existing grammar\.

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
