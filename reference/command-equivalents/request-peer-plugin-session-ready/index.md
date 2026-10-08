# `request peer plugin session ready`

Signal that per\-peer plugin setup is complete\.

## Ze command

- Registry path: `request peer plugin session ready`
- Usage: `request peer <selector> plugin session <session> ready`
- Mode: Daemon
- Wire method: `ze-bgp:plugin-session-peer-ready`
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

The daemon closes this sending process\'s share of the peer\'s live\-forward replay fence only for the captured peer\-UP session token\. Missing\, stale or retired process receipts fail without releasing queued work\. An explicit operator request is a no\-op\; it cannot credit a named process\. A peer of \'\*\'\, and an empty peer\, signal nothing\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | Peer selector | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, or \* for every peer\. \'pause\' and \'resume\' refuse a selector that matches more than one peer\, and the other commands act on each peer it matches\. |
| `session` | string | no | any value of this type | Captured peer\-UP initial\-replay token\. | The opaque nonzero uint64 initial\-replay token captured from the peer\-UP event whose replay this process finished\. Never fetch a new token to finish old replay work\. |

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
