# `show announcements`

List active on\-demand announcements\.

## Ze command

- Registry path: `show announcements`
- Usage: `show announcements [tag <tag>] [selector <selector>] [family <family>]`
- Mode: Read-only
- Wire method: `ze-bgp:show-announcements`
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

Each row carries the id\, the tag\, the family\, the peer selector\, the source and the creation time\. An expiry time is present only when the announcement was made with for\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `tag` | string | no | any value of this type | Show only announcements carrying this tag key | The value is a tag key\, compared as text with the key each announcement was made with\. Absent\, announcements of every key are listed\. |
| `selector` | string | no | any value of this type | Show only announcements whose peer selector matches this pattern | The value is compared as text with the selector the announcement was made with\, so it MUST be spelled the way that announcement spelled it\. Absent\, announcements to every selector are listed\. |
| `family` | string | no | any value of this type | Show only announcements of this address family | The value is an address family as the family column reports it\, such as ipv4\/unicast\. Absent\, announcements of every family are listed\. |

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
