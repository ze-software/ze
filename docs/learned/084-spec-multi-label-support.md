# 084 — Multi-Label Support

## Objective

Extend MPLS label fields from single `uint32` to `[]uint32` throughout the config, static route, and wire encoding path to support RFC 8277 label stacks.

## Decisions

- Reused existing `nlri.EncodeLabelStack()` for wire encoding instead of the manual single-label encoding: the helper already handles BOS bit correctly for multiple labels.
- `totalBits` calculation must account for N labels: `len(labels)*24 + [64 for VPN] + prefixBits` — easy to miss when migrating from hardcoded 24.
- Config supports both `label 100` (old, single) and `labels [100 200]` (new, multiple): backward-compatible migration.
- Capability negotiation (RFC 8277 Multiple Labels Capability) deferred: config-originated routes are operator responsibility; enforcement can come later.
- VPN routes with empty labels validated in config loader (fail early): `IsVPN()` only checks RD, so an empty-label VPN would produce invalid wire format without explicit validation.
- `IsLabeledUnicast()` self-validates via `len(Labels) > 0`: empty labels → treated as plain unicast, no explicit check needed.

## Gotchas

- `totalBits` is the most dangerous field to migrate: changing from `24 + prefixBits` to `len(labels)*24 + prefixBits` silently produces wrong wire format if missed.
- Breaking change: existing VPN configs that omitted `label` (implicitly label 0) now error. Explicit `label 0` required.

## Files

- `internal/reactor/peersettings.go` — StaticRoute.Labels []uint32
- `internal/bgp/message/update_build.go` — VPNParams.Labels, LabeledUnicastParams.Labels, wire encoding
- `internal/component/config/loader.go`, `bgp.go` — labels array syntax + backward compat
