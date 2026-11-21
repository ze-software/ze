# `show bgp peer rib`

Show RIB data scoped to one peer\.

## Ze command

- Registry path: `show bgp peer rib`
- Usage: `show bgp peer <selector> rib`
- Mode: Read\-only
- Wire method: `ze-bgp:peer-rib`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: peer\, next\-hop
- Column order: peer\, direction\, family\, prefix\, next\-hop\, path\-id\, as\-path\, origin\, local\-pref\, med\, communities
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill\, resolve\, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: `advertised`: Select advertised routes; `aspath <value>`: Filter by AS path\, another spelling of path; `community <value>`: Filter by standard community; `count`: Count matching routes without serializing rows; `covering <value>`: Received routes whose prefix contains an IP address; `exact <value>`: Received routes of exactly one prefix; `family <value>`: Filter by AFI\/SAFI; `first <value>`: Take first N routes; `graph`: Render AS\-path topology graph; `histogram`: Count routes by family and prefix length; `last <value>`: Take last N routes; `match <value>`: Cross\-field structured match; `path <value>`: Filter by AS path; `peer <value>`: Filter by peer; `prefix <value>`: Filter by prefix; `received`: Select received routes; `sent`: Select advertised routes\, another spelling of advertised; `sent-received`: Select received and advertised routes\, the default
- Pipe aliases: none

The command forwards to the RIB plugin with the peer selector attached\, so the RIB plugin MUST be loaded for it to answer\. \'show bgp rib\' reads the same plugin without a peer scope\. The direction is a pipe operation\: \'\| received\' reads the routes the peer sent to Ze\, \'\| sent\' or \'\| advertised\' the routes Ze sent to it\, and \'\| sent\-received\'\, the default\, both\. The other pipe operations of \'show bgp rib\' apply too\, such as \'\| prefix\'\, \'\| count\' and \'\| first\'\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | \* for every peer\, or an address\, a peer name or an AS pattern | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, or \* for every peer\. Every command under \'show bgp peer\' reads the peers it matches\, except \'list\'\, which reads every peer\. |

## Mapping intents

### Routes received from one BGP peer

Category: BGP

### Routes advertised to one BGP peer

Category: BGP

Ze uses pipe filters on the peer RIB command for received and advertised views\.

## Vendor equivalents

### Junos MX
- `show route advertising-protocol bgp <peer>` (verified, junos\-route)
  - Intent: Routes advertised to one BGP peer
- `show route receive-protocol bgp <peer>` (verified, junos\-route)
  - Intent: Routes received from one BGP peer

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
