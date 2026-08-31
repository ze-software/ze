# `show pki certificate name fingerprint`

Show the hash of the certificate, to verify its identity against the one another system reports.

## Ze command

- Registry path: `show pki certificate name fingerprint`
- Usage: `show pki certificate name <name> fingerprint [sha256\|sha384\|sha512]`
- Mode: Read-only
- Wire method: `ze-show:pki-certificate-fingerprint`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `algorithm`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Without an algorithm, SHA-256.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `name` | string | yes | any value of this type |

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
