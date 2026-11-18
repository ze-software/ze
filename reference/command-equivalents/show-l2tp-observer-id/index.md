# `show l2tp observer id`

Select one original session\'s event history\.

## Ze command

- Registry path: `show l2tp observer id`
- Usage: `show l2tp observer id <id> [expected-id <expected-id>] [listener <listener>] [tunnel-id <tunnel-id>]`
- Mode: Read\-only
- Wire method: `ze-l2tp:observer`
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

Read the bounded event ring of one captured original\. Qualify a shared session ID with tunnel\-id and listener\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `id` | uint | yes | any value of this type | Local session ID\. | The local session ID whose original event ring is wanted\. |
| `expected-id` | string | no | any value of this type | Require the displayed immutable session reference\. | Require the displayed immutable original reference\. An ended original refuses instead of selecting a replacement with the same numeric IDs\. |
| `listener` | string | no | any value of this type | Configured listener name or bound IP\:port\. | Configured listener name or bound IP\:port from the listener inventory\. Equal session and tunnel IDs on different listeners require this qualifier\. |
| `tunnel-id` | uint | no | any value of this type | Original local tunnel ID\. | Restrict selection to the local tunnel carrying the original session\. A session ID shared by several tunnels requires this qualifier\. |

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
