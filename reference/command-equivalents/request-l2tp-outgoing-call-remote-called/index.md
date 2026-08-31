# `request l2tp outgoing-call remote called`

Place an LNS-side outgoing call (RFC 2661 S10.4).

## Ze command

- Registry path: `request l2tp outgoing-call remote called`
- Usage: `request l2tp outgoing-call remote <remote> called <called>`
- Mode: Daemon
- Wire method: `ze-l2tp-api:outgoing-call`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Dials the named remote (which must have outgoing-calls enabled), sends OCRQ, and blocks until the call establishes or fails. On failure the cause and RFC 2661 Result Code are reported (auth reject, tie-breaker loss, peer CDN, or timeout).

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `remote` | string | yes | any value of this type |
| `called` | string | yes | any value of this type |

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
