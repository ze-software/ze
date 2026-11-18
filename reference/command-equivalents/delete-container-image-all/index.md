# `delete container image all`

Delete every stored image no configured container holds\.

## Ze command

- Registry path: `delete container image all`
- Usage: `delete container image all`
- Mode: Daemon
- Wire method: `ze-container:delete-image-all`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Every stored image no configured container holds is removed\, with the blobs a stopped fetch left\. An image a container holds is kept and listed under kept\, with the container that holds it\.

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
