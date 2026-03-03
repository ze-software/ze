# 280 — Capability Mode (require/refuse Enforcement)

## Objective

Add `require` and `refuse` enforcement modes to all BGP capabilities, unifying the config vocabulary with the existing address family mode system. Primary use case: `asn4 require;` to reject peers that don't support 32-bit ASNs.

## Decisions

- Enforcement is post-negotiation: exchange OPENs, negotiate the intersection, then check requirements. This avoids modifying the wire exchange — the NOTIFICATION (Unsupported Capability) is sent after both OPENs are received and processed.
- `refused` capabilities require checking peer's raw OPEN (not the negotiated set): negotiation is an intersection — a refused capability won't appear in the negotiated result. `peerCodes map[Code]bool` stored in `Negotiated` during `Negotiate()` provides access to the peer's raw advertised codes.
- Created separate `buildUnsupportedCapabilityDataCodes()` for non-family capability codes rather than generalizing the existing family function: simpler — no interface changes needed.
- Kept `asn4` as YANG `type boolean`, extended `TypeBool` validator to accept `require`/`refuse` alongside `true/false/enable/disable`: avoids a YANG type change that would break serializer roundtrip and existing tests.
- `capModeTokens` as single source of truth: `isCapModeToken()` derives via `slices.Contains()` — no separate switch statement to maintain in sync.
- Used `.ci` file options (`drop-capability`/`add-capability`) to control ze-peer's OPEN capabilities for functional tests: simpler than in-band cap 254 signaling.

## Patterns

- `validateCapabilityModes()` extracted as a shared helper called in both `processOpen()` and `handleOpen()`. Both OPEN processing paths must check capability modes — missing one path leaves a hole.
- Trailing mode token detection for add-path per-family: if the last token is a mode word, it's a mode, otherwise it's a direction. `isCapModeToken()` makes this unambiguous.

## Gotchas

- Absent opt-in capabilities were parsed as `enable` because `flexString("")` returns `""` and `parseCapMode("")` defaulted to enable. Fixed by checking `v != ""` before calling `parseCapMode`.
- Add-path per-family require (AC-8/9) is code-level enforcement only — not per-family granularity. If `ipv4/unicast send require` but peer only offers `ipv6/unicast add-path`, the session is rejected (code 69 not in peer's OPEN). True per-family add-path enforcement would need a separate mechanism.
- `asn4 true` → `enable` and `asn4 false` → `disable` backwards compatibility must be preserved via the `TypeBool` extension. Changing the YANG type to string broke two existing tests and required revert.

## Files

- `internal/plugins/bgp/reactor/config.go` — `capMode`, `applyCapMode`, `parseCapabilitiesFromTree`, `parseAddPathFromTree` with mode support
- `internal/plugins/bgp/reactor/peersettings.go` — `RequiredCapabilities`, `RefusedCapabilities` fields
- `internal/plugins/bgp/capability/negotiated.go` — `peerCodes`, `CheckRequiredCodes`, `CheckRefusedCodes`
- `internal/plugins/bgp/reactor/session.go` — `validateCapabilityModes` helper, enforcement in `processOpen`/`handleOpen`
- `internal/component/config/schema.go` — `TypeBool` accepts require/refuse
- `internal/test/peer/peer.go` — `CapabilityOverride`, `applyCapabilityOverrides`, drop/add-capability
- `test/encode/cap-require-asn4.ci`, `test/encode/cap-refuse-asn4.ci` — functional tests
