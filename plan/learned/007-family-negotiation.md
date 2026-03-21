# 007 — Family Negotiation

## Objective

Extend family configuration to support per-family modes (enable, disable, require, ignore) and reject BGP sessions where a required family is not negotiated by the peer.

## Decisions

- Four modes chosen over three: `enable`, `disable`, `require`, `ignore` — `ignore` was added during implementation as a fourth mode for per-family UPDATE leniency, going beyond the original plan.
- Both inline (`ipv4/unicast require;`) and block (`ipv4 { unicast require; }`) syntaxes supported simultaneously — neither was removed.
- `require` overrides `ignore-mismatch` global flag — the stricter setting always wins.
- NOTIFICATION sent with Error Code 2, Subcode 7 (Unsupported Capability) per RFC 5492 §3 when a required family is missing.
- `require` and `ignore` are Ze extensions — ExaBGP does not support these modes.

## Patterns

- Pattern matching for templates extended to support CIDR notation (`10.0.0.0/8`) and IPv6 glob patterns (`2001:db8::*`) in the same pass.

## Gotchas

- None documented.

## Files

- `internal/component/config/bgp.go` — FamilyMode, FamilyConfig, FamilyModeIgnore, schema and parsing
- `internal/component/config/loader.go` — Convert FamilyConfig to capabilities and RequiredFamilies
- `internal/bgp/capability/negotiated.go` — CheckRequired()
- `internal/reactor/session.go` — validate required families post-negotiate
