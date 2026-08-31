# `request ospf graceful-restart`

Trigger a planned OSPFv2 graceful restart (RFC 3623 section 2.1).

## Ze command

- Registry path: `request ospf graceful-restart`
- Usage: `request ospf graceful-restart`
- Mode: Daemon
- Wire method: `ze-ospf:graceful-restart-prepare`
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

The engine originates one Grace-LSA per interface, persists the non-volatile restart fact, and suppresses route churn so the FIB is retained across the ensuing control-plane restart. Refused when graceful-restart is not configured.

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
