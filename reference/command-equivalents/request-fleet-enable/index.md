# `request fleet enable`

Rejoin the fleet\: follow the hub again\.

## Ze command

- Registry path: `request fleet enable`
- Usage: `request fleet enable`
- Mode: daemon\, offline
- Wire method: `ze-managed:fleet-enable`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, when the answer has rows: match\, count\, first\, last\, display\, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Stores the managed flag as true\. A daemon that started managed then starts its hub client from the configuration it runs\, with no restart\; a daemon that started unmanaged connects to its hub at the next ze start\, and the answer says so\. The command is refused\, and the flag left as it was\, while the configuration names no hub\. It takes no argument\. With no daemon running\, ze request fleet enable stores the flag\, and the next ze start follows it\.

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
