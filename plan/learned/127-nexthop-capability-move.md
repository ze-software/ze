# 127 — Nexthop Capability Move

## Objective

Move the `nexthop` block from peer level into the `capability` block, since it configures RFC 8950 Extended Next Hop capability families and logically belongs with other capabilities.

## Decisions

Mechanical refactor, no design decisions.

## Patterns

- Freeform parsing naturally handles `ipv4/unicast ipv6;` family syntax inside the nexthop block without extra parser logic.
- Recursive serialization requires no changes when moving a block deeper in the tree.
- Migration for ExaBGP configs reads nexthop at neighbor level (ExaBGP location) and writes it inside capability (Ze BGP location).

## Gotchas

None.

## Files

- `internal/component/config/bgp.go` — nexthop moved from peer schema to capability schema, parsing updated in both `applyTreeSettings` and `parsePeerConfig`
- `internal/exabgp/migrate.go` — `migrateCapability()` now places nexthop block inside capability
- Config files: `etc/ze/bgp/*.conf`, `test/data/encode/`, `test/data/plugin/`
