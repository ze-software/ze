# `show pki local-ca pem`

Export the root certificate of the local certificate authority as PEM.

## Ze command

- Registry path: `show pki local-ca pem`
- Usage: `show pki local-ca pem`
- Mode: Read-only
- Wire method: `ze-show:pki-local-ca-pem`
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

Give this text to each client that must trust this node, in its `pki ca <name> certificate` block. The output holds the certificate only: the root private key never leaves the daemon.

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
