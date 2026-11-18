# `update container image`

Update a container\'s image\: name \<container\>\, or rollback\.

## Ze command

- Registry path: `update container image`
- Usage: `update container image name <container>`
- Mode: Daemon
- Wire method: `not listed`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: `name`, `rollback`
- Answer shape: doc
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

An update needs the name of one container\, after the name keyword\: update container image name \<container\>\. rollback returns it to its previous image\.

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
