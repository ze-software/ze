# 006 — Align Implementation

## Objective

Implement 26 ExaBGP alignment items plus 5 additional violations found during RFC annotation, bringing Ze to full RFC compliance across capabilities, attributes, NLRI types, and error handling.

## Decisions

- RFC 9072 extended optional parameters: auto-detected (pack extended format when params > 255 bytes) rather than negotiated separately.
- EVPN Type 5 prefix encoding fixed to use fixed 4/16 byte fields per RFC 9136 §3.1 (was variable-length, which was wrong).
- BGP-LS descriptor TLVs removed from container wrapping — they now appear directly in the NLRI body per RFC 7752 §3.2.
- `countASNs()` for AS4_PATH merge now counts AS_SET=1 and confederation segments=0, per RFC 4271 §9.1.2.2.
- AS_CONFED segments from OLD speakers (no ASN4 capability) are now discarded per RFC 6793 §3.

## Patterns

- Each alignment item was implemented with a test first (TDD), then wired into the dispatch path.
- Phase organization: Critical → Capabilities → Timers → Attributes → MP-NLRI → NLRI Types → Error handling → Config.

## Gotchas

- Five violations were discovered during annotation (spec 005) and added to this spec — the original 26-item list was incomplete.
- `validateFamilies()` in session.go validates against negotiated families; a config option for buggy peers was added to override strict validation.

## Files

- `internal/bgp/message/` — notification.go, open.go, header.go, keepalive.go, routerefresh.go
- `internal/bgp/attribute/` — as4.go, aspath.go, community.go, origin.go
- `internal/bgp/nlri/` — evpn.go, bgpls.go, flowspec.go, other.go
- `internal/bgp/capability/` — negotiated.go
- `internal/component/config/bgp.go` — hold time, local address auto, extended-message, per-family add-path
