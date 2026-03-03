# 125 — ExaBGP Compatibility

## Objective

Provide two migration tools: `ze exabgp plugin` (runtime bridge wrapping ExaBGP plugins to work with Ze BGP's 5-stage protocol) and `ze bgp exabgp migrate` (one-time config converter from ExaBGP to Ze BGP format).

## Decisions

- Bridge handles 5-stage startup internally on behalf of the ExaBGP plugin, which knows nothing about Ze's protocol. After `ready`, switches to JSON translation mode.
- Scanner is reused between startup and JSON phases to prevent buffered data loss at the phase transition.
- Two separate migration systems exist: `internal/config/migration/` (Ze BGP internal syntax evolution) and `internal/exabgp/migrate.go` (ExaBGP→Ze BGP conversion). These are separate concerns.
- RIB plugin auto-injected when ExaBGP config has graceful-restart or route-refresh: Ze BGP delegates RIB to plugins, so these features require an explicit RIB plugin.
- Chose native `log/syslog` over external slog-syslog library to avoid go.mod changes.

## Patterns

- ExaBGP uses space-separated families (`ipv4 unicast`); Ze BGP uses slash (`ipv4/unicast`). Conversion is systematic.
- ExaBGP has TWO nexthop-related configs: `capability { nexthop; }` (enable flag) AND `nexthop { }` block (AFI/SAFI tuples). Both must be detected for capability inference.

## Gotchas

- The ExaBGP schema (`internal/exabgp/schema.go`) is distinct from the Ze BGP schema — ExaBGP uses `api { processes [...] }`, not Ze BGP's `process { processes [...] }`. Using the wrong schema causes `migrateAPIBlock()` to silently never execute.
- Test data for migration initially used Ze BGP syntax instead of actual ExaBGP syntax — tests passed but didn't test real ExaBGP migration.
- VPN-IPv4 with IPv6 next-hop: original code had `case 40` but correct size is 8+16+8+16=48. Critical parsing bug found during review.

## Files

- `internal/exabgp/bridge.go` — JSON/command translation, startup protocol, `Bridge` struct
- `internal/exabgp/migrate.go` — ExaBGP→Ze BGP migration logic
- `internal/exabgp/schema.go` — ExaBGP-specific config schema (needed for `api` block)
- `cmd/ze/bgp/exabgp.go` — CLI: `ze exabgp plugin`, `ze bgp exabgp migrate`
