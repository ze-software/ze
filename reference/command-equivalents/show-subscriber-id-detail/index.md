# `show subscriber id detail`

Show everything about one subscriber session\.

## Ze command

- Registry path: `show subscriber id detail`
- Usage: `show subscriber id <id> detail`
- Mode: Read-only
- Wire method: `ze-l2tp:subscriber-detail`
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

Pass the session ID\. Returns access type\, assigned addresses\, authentication state\, uptime\, and traffic counters\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `id` | string | yes | any value of this type | Subscriber session ID\. | The id column of the show subscriber table\. An L2TP session is named l2tp\-\<tunnel\-id\>\-\<session\-id\> and a PPPoE session pppoe\-\<ifindex\>\-\<session\-id\>\. An unknown id is refused with \'session not found\'\. |

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
