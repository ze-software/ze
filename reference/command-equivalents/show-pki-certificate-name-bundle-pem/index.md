# `show pki certificate name bundle pem`

Export the certificate, its intermediates and its private key as one PEM stream.

## Ze command

- Registry path: `show pki certificate name bundle pem`
- Usage: `show pki certificate name <name> bundle pem`
- Mode: Read-only
- Wire method: `ze-show:pki-certificate-bundle-pem`
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

Device certificates only: a CA certificate in the store holds no private key.

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
