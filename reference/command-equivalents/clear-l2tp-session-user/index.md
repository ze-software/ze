# `clear l2tp session user`

Disconnect the subscriber session of one user name\.

## Ze command

- Registry path: `clear l2tp session user`
- Usage: `clear l2tp session user`
- Mode: Daemon
- Wire method: `ze-l2tp:session-teardown-user`
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

Clears the one live session whose user name equals the given name exactly\: clear l2tp session user \<name\> \[actor \<label\>\] \[reason \<text\>\] \[cause \<code\>\]\. When several sessions carry the name\, nothing is cleared and the refusal lists one clear l2tp session id command for each\, as a RADIUS Disconnect\-Request matching several sessions is refused\.

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
