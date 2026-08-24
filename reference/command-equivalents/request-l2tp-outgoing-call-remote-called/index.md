# `request l2tp outgoing-call remote <name> called <number>`

## Ze command

- Syntax: `request l2tp outgoing-call remote <name> called <number>`
- Registry path: `request l2tp outgoing-call remote called`
- Mode: Daemon
- Wire method: `ze-l2tp-api:outgoing-call`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Place an LNS-side outgoing call (RFC 2661 S10.4). Usage: request l2tp outgoing-call remote <name> called <number>. Dials the named remote (which must have outgoing-calls enabled), sends OCRQ, and blocks until the call establishes or fails. On failure the cause and RFC 2661 Result Code are reported (auth reject, tie-breaker loss, peer CDN, or timeout).

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
