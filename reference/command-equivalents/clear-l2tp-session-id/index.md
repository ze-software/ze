# `clear l2tp session id`

Disconnect one subscriber session\.

## Ze command

- Registry path: `clear l2tp session id`
- Usage: `clear l2tp session id`
- Mode: Daemon
- Wire method: `ze-l2tp:session-teardown`
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

Captures one original session\, sends its CDN and confirms its local physical cleanup before reporting success\. Pass the local session ID\: clear l2tp session id \<id\> \[expected\-id \<birth\>\] \[tunnel\-id \<tid\>\] \[listener \<name\-or\-endpoint\>\] \[actor \<label\>\] \[reason \<text\>\] \[cause \<code\>\]\. The optional expected\-id is the canonical immutable reference from the displayed session\; a stale reference refuses before audit or mutation\. A listener name resolves to its original bound endpoint\. Unknown names and ambiguous selections are refused\. Quote reasons that contain another keyword\.

## Arguments

No command-specific arguments listed.

## Mapping intents

### L2TP tunnel and session state

Category: VPN and access

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
