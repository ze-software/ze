# 118 — NLRI JSON Event Format (Command Style)

## Objective

Change the UPDATE event NLRI JSON format from ExaBGP-style (announce/withdraw with next-hop grouping) to command style (`family → [{"action": "add"/"del", "next-hop": "...", "nlri": [...]}]`).

## Decisions

- Family value is always an array; each element is an operation object with explicit `action` field.
- Grouped by next-hop within a family — one UPDATE can carry multiple MP_REACH_NLRI with different next-hops for the same family (RFC 4760 § 3); grouping avoids duplication.
- Withdrawals omit `next-hop`; announcements include it.
- Simple prefix families (unicast, multicast): NLRI items are strings when ADD-PATH is off, objects when on.
- RD format gains type prefix: `0:65000:100`, `1:192.0.2.1:100`, `2:65000:1` — Type 0 and Type 2 were otherwise indistinguishable.
- Breaking change: all plugins parsing JSON events need updates. No compat flag — clean break.

## Patterns

- `json:` lines in `.ci` files are documentation only — the test framework (`testpeer`) does NOT validate them. True JSON functional tests require extending the test infrastructure (deferred).

## Gotchas

- RD String() was missing the type prefix — bug found and fixed during this work. `internal/bgp/nlri/ipvpn.go` now outputs `0:`, `1:`, `2:` prefix.
- FlowSpec JSON uses `String()` (human-readable) rather than structured operators — deferred to a future spec. Operators not yet exposed as structured data.

## Files

- `internal/plugin/text.go` — `formatFilterResultJSON()`, `familyOperation` struct, family-specific formatters
- `internal/plugin/rib/event.go` — `FamilyOperation` struct for new format parsing
- `internal/bgp/nlri/ipvpn.go` — RD String() with type prefix
- `docs/architecture/api/architecture.md` — updated JSON format examples
- `docs/exabgp/exabgp-migration.md` — migration guide (renamed from `exabgp-compatibility.md`)
