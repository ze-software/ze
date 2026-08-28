# `show pki certificate name <name> [pem | bundle pem | fingerprint`

## Ze command

- Syntax: `show pki certificate name <name> [pem \| bundle pem \| fingerprint`
- Registry path: `show pki certificate name`
- Mode: Read-only
- Wire method: `ze-show:pki-certificate`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Inspect a specific certificate in detail. Usage: show pki certificate name <name> [pem | bundle pem | fingerprint [sha256|sha384|sha512]]. Use 'pem' to export for another system, 'fingerprint' to verify identity.

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
