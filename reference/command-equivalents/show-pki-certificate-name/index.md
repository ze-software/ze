# `show pki certificate name`

Inspect a specific certificate in detail\.

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
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Each export form is a command of its own\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Certificate name | The name of a CA or device certificate in the store\. An unknown name is refused\, and the refusal lists the names the store holds\. |

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
