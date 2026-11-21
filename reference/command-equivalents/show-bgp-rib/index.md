# `show bgp rib`

Query routes in the BGP RIB\.

## Ze command

- Registry path: `show bgp rib`
- Usage: `show bgp rib`
- Mode: Read\-only
- Wire method: `ze-bgp:rib-routes`
- Backends: any backend
- Task support: required\: the MCP server always answers with a task handle
- Subcommands: `best`, `protocol`, `rpf`, `status`
- Answer shape: tab
- Address fields: peer\, next\-hop
- Column order: peer\, direction\, family\, prefix\, next\-hop\, path\-id\, as\-path\, origin\, local\-pref\, med\, communities
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill\, resolve\, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: `advertised`: Select advertised routes; `aspath <value>`: Filter by AS path\, another spelling of path; `community <value>`: Filter by standard community; `count`: Count matching routes without serializing rows; `covering <value>`: Received routes whose prefix contains an IP address; `exact <value>`: Received routes of exactly one prefix; `family <value>`: Filter by AFI\/SAFI; `first <value>`: Take first N routes; `graph`: Render AS\-path topology graph; `histogram`: Count routes by family and prefix length; `last <value>`: Take last N routes; `match <value>`: Cross\-field structured match; `path <value>`: Filter by AS path; `peer <value>`: Filter by peer; `prefix <value>`: Filter by prefix; `received`: Select received routes; `sent`: Select advertised routes\, another spelling of advertised; `sent-received`: Select received and advertised routes\, the default
- Pipe aliases: none

Look at received or advertised routes\, each choice a pipe operation typed after the command\: the scope \| received\, \| advertised \(or \| sent\) or \| sent\-received\; the filters \| peer\, \| family\, \| prefix \(a text match that can also list longer prefixes\)\, \| path \(or \| aspath\)\, \| community\, \| match\, \| covering\, which lists the received routes whose prefix contains an IP address\, and \| exact\, which lists the received routes of one prefix\; then \| count\, \| first\, \| last\, \| histogram or \| graph\. This is the main route inspection command\.

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
