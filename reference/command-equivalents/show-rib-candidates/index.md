# `show rib candidates`

Show every path each protocol offered the Loc\-RIB\, hidden ones included\, with the import chain that hid each\.

## Ze command

- Registry path: `show rib candidates`
- Usage: `show rib candidates prefix <prefix>`
- Mode: Read-only
- Wire method: `not listed`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists\, per prefix\, every path a protocol offered\: the selected one\, the ones standing by\, and the ones an import chain hid\. Each path shows its source\, next hop\, distance and metric\, its tag\, IS\-IS level or OSPF route type when it carries one\, and for a hidden path the chain that hid it\: \`source\` \(the protocol\'s own \`filter import\`\) or \`central\` \(\`rib filter import\`\)\. \`prefix \<prefix\>\` limits the answer to one prefix\.

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
