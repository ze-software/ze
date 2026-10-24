# `request bgp watchdog give`

Give a route to a watchdog\, sent now or held\.

## Ze command

- Registry path: `request bgp watchdog give`
- Usage: `request bgp watchdog give <name> <owner>`
- Mode: Daemon
- Wire method: `ze-bgp:watchdog-give`
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

A process gives one route to a watchdog\. The ExaBGP bridge sends it for an announce that names a watchdog\. The route is sent now\, or with withdraw held until \'request bgp watchdog announce \<name\>\'\. The same route given again replaces the first\. The command reads\: give \<name\> owner \<owner\> selector \<selector\> \[withdraw\] announce \<n\> \<update text\> withdrawal \<update text\> where n counts the words of the announce text\, because update text has no word that cannot appear in it\. A route the engine refuses to send is not kept\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Watchdog name | The watchdog the route is given to\. med is refused as a name\. |
| `owner` | string | yes | any value of this type | Owner\, selector\, announce and withdrawal of the route | The owner\, then selector \<selector\>\, an optional withdraw\, announce \<n\> \<update text\> and withdrawal \<update text\>\. The owner names the process that gave the route\, so \'request bgp watchdog forget \<owner\>\' takes it back\. |

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
