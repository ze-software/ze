# `request fleet disable`

Leave the fleet\: end the hub connection and stop following the hub\.

## Ze command

- Registry path: `request fleet disable`
- Usage: `request fleet disable`
- Mode: daemon\, offline
- Wire method: `ze-managed:fleet-disable`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
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

Stores the managed flag as false\, then ends the hub connection of a daemon that started managed\, and answers once that connection has ended\. A configuration the device was applying is applied first\. The device keeps the configuration it runs\, and its hub can no longer change it\. The command needs no hub and no connection\, and takes no argument\. With no daemon running\, ze request fleet disable stores the flag\, and the next ze start follows it\.

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
