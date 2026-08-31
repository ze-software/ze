# `show pki certificates`

List all loaded certificates with expiry dates.

## Ze command

- Registry path: `show pki certificates`
- Usage: `show pki certificates`
- Mode: Read-only
- Wire method: `ze-show:pki-certificates`
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

Shows name, type (CA or device), subject, issuer, expiry, and validity status. Check here to find certificates approaching expiration.

## Arguments

No command-specific arguments listed.

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
