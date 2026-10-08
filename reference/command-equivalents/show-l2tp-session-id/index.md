# `show l2tp session id`

Show full detail for one L2TP session\.

## Ze command

- Registry path: `show l2tp session id`
- Usage: `show l2tp session id <id>`
- Mode: Read-only
- Wire method: `ze-l2tp:session`
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

Pass the local session ID\. Returns PPP state\, assigned addresses\, negotiated LCP\/NCP options\, and traffic counters\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `id` | string | yes | any value of this type | Local session ID\. | The decimal local session ID from the show l2tp session table\. Zero is refused because RFC 2661 reserves it\, and an unknown ID is refused with \'no session with local\-sid\'\. |

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
