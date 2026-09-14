# `update bgp config`

Write the running peer set to the configuration file.

## Ze command

- Registry path: `update bgp config`
- Usage: `update bgp config`
- Mode: Daemon
- Wire method: `ze-bgp:peer-save`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Column order: added, removed, config, message
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Ze writes the running peer set into the configuration file. A peer 'create bgp peer' built is added to the file, and a peer the file declares that 'delete bgp peer' removed is taken out of it. A peer both sides already hold is left exactly as it was written. The running daemon does not change: the file is brought to it. The command takes no selector, because after a delete no peer is left to name and that absence is half of what the command persists. A created peer is written under a name derived from its address, such as 'peer-192.0.2.7', because a peer name cannot start with a digit. The answer names what was added, what was removed, and the file that was written.

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
