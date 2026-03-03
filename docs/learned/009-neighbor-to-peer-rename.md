# 009 — Neighbor-to-Peer Rename

## Objective

Unify BGP session terminology by renaming `neighbor` to `peer` and restructuring templates to separate named groups (`template.group`) from glob patterns (`template.match`), removing the ambiguity where `peer` at root level meant a glob and `neighbor` meant both templates and sessions.

## Decisions

- `match` blocks apply in **config-file order**, not by specificity — this was explicitly chosen to give operators full control over application order (general→specific or specific→general both valid).
- `inherit` inside `template {}` is explicitly rejected with a clear error — inheritance only applies to `peer {}` blocks.
- `match` only valid inside `template {}` (not at root or inside `peer {}`) — enforced at parse time.
- Group names must start with a letter and not end with a hyphen — validation enforced to prevent ambiguity with IP addresses.
- Static routes can appear in both `match` and `group` blocks, enabling routes announced to all peers (via `match *`) or only when explicitly inherited (via `group`).
- Migration preserves insertion order of `match` blocks — critical because order determines application semantics.

## Patterns

- Precedence: `template.match` (config order) → `template.group` (inherit order) → `peer` level. Each overrides the previous.

## Gotchas

- Multiple `inherit` statements within one `peer {}` were planned but only single `inherit` was initially implemented — noted as a known limitation in Phase 1.
- IPv6 glob patterns (`2001:db8::*`) and CIDR notation (`10.0.0.0/8`) both supported as match patterns.

## Files

- `internal/component/config/bgp.go` — schema changes, PeerConfig rename, match/group parsing
- `internal/component/config/migration/` — v2_to_v3 migration (neighbor-to-peer)
- `internal/component/config/serialize.go` — outputs v3 format
