# `show pki certificate name`

Inspect a specific certificate in detail.

## Ze command

- Registry path: `show pki certificate name`
- Usage: `show pki certificate name <name>`
- Mode: Read-only
- Wire method: `ze-show:pki-certificate`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `bundle`, `fingerprint`, `pem`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Each export form is a command of its own.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `name` | string | yes | any value of this type |

## Mapping intents

### Certificate inventory

Category: Security

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
