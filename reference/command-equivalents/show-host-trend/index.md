# `show host trend`

Show host resources heading for exhaustion\.

## Ze command

- Registry path: `show host trend`
- Usage: `show host trend`
- Mode: Read\-only
- Wire method: `ze-host:show-trend`
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

Lists every host resource rule the daemon\'s trend monitor runs\, such as kernel\-memory\. Each entry gives the rule\'s state \(not\-present\, warming\-up\, quiet\, confirming\, active or unreadable\)\, its level and limit with their unit\, how long it has been observed\, the growth over the window it judged\, and the projected time to exhaustion when one exists\. An active rule also carries the warning it raised\. The trend monitor runs only inside the daemon\, so this command needs a running daemon\. It is not part of \'show host all\'\.

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
