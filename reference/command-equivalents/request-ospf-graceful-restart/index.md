# `request ospf graceful-restart`

## Ze command

- Syntax: `request ospf graceful-restart`
- Registry path: `request ospf graceful-restart`
- Mode: Daemon
- Wire method: `ze-ospf:graceful-restart-prepare`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Trigger a planned OSPFv2 graceful restart (RFC 3623 section 2.1). Usage: request ospf graceful-restart. The engine originates one Grace-LSA per interface, persists the non-volatile restart fact, and suppresses route churn so the FIB is retained across the ensuing control-plane restart. Refused when graceful-restart is not configured.

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
