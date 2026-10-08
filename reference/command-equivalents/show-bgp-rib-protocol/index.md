# `show bgp rib protocol`

Query the routes one protocol feeds into the RIB\.

## Ze command

- Registry path: `show bgp rib protocol`
- Usage: `show bgp rib protocol <protocol>`
- Mode: Read-only
- Wire method: `ze-bgp:rib-protocol`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: peer, next-hop
- Column order: peer, direction, family, prefix, next-hop, path-id, as-path, origin, local-pref, med, communities
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: `advertised`: Select advertised routes; `community <value>`: Filter by standard community; `count`: Count matching routes without serializing rows; `family <value>`: Filter by AFI\/SAFI; `first <value>`: Take first N routes; `graph`: Render AS\-path topology graph; `histogram`: Count routes by family and prefix length; `last <value>`: Take last N routes; `match <value>`: Cross\-field structured match; `path <value>`: Filter by AS path; `peer <value>`: Filter by peer; `prefix <value>`: Filter by prefix; `received`: Select received routes
- Pipe aliases: none

Answers the same rows as show bgp rib\, limited to the Adj\-RIB\-In tables one protocol feeds\. bgp reads the BGP peers\. Every other protocol\, bmp for example\, reads the tables its monitored peers fill\. The pipeline words after the protocol are the ones show bgp rib takes\: peer \<selector\> limits the answer to the matching peers\, then filters\, then one terminal\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `protocol` | string | yes | any value of this type | Registered protocol name | The name of a registered protocol\. The bgp\-rib plugin refuses a name the protocol registry does not hold and names the registered ones\. |

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
