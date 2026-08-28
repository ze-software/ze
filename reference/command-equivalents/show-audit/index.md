# `show audit [<action>] [<actor>] [<count>] [<since>] [<surface>] [<until>]`

## Ze command

- Syntax: `show audit [<action>] [<actor>] [<count>] [<since>] [<surface>] [<until>]`
- Registry path: `show audit`
- Mode: Read-only
- Wire method: `ze-show:audit`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show who did what and when on this box. Returns audit log entries with timestamps, actors, and actions. Filters (all optional, combinable): action <type>, actor <name>, surface <name> (cli, web, api), since/until <RFC3339>, count <N>. Actions include config-commit, login, peer-teardown, and more.

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
