# Spec: capability-mode

## Task

Added `require` and `refuse` enforcement modes to all BGP capabilities, unifying the config vocabulary with the existing family mode system. Previously only address families supported `require`; non-family capabilities (ASN4, route-refresh, extended-message, add-path, graceful-restart, extended-next-hop) accepted only enable/disable with no enforcement.

**Primary use case:** `asn4 require;` — reject peers that don't support 32-bit ASNs.

### Unified Mode Vocabulary

All capabilities and families share a four-mode vocabulary:

| Mode | Advertise? | Enforcement |
|------|------------|-------------|
| `enable` | Yes | None — proceed if peer lacks it |
| `disable` | No | None — proceed either way |
| `require` | Yes | Reject session if peer **lacks** it |
| `refuse` | No | Reject session if peer **has** it |

### Config Syntax

**Simple capabilities** — mode is the value:

```
capability {
    asn4 require;
    extended-message enable;
    route-refresh require;
    software-version disable;
}
```

**Block capabilities** — `mode` key inside block:

```
capability {
    graceful-restart {
        mode require;
        restart-time 120;
    }
    nexthop {
        mode enable;
        ipv4/unicast ipv6;
    }
}
```

**Add-path** — mode pushed down per-family (like family require):

```
capability {
    add-path send/receive require;        # global: all families
}

add-path {
    ipv4/unicast send require;            # per-family with mode
    ipv6/unicast send/receive;            # per-family, default enable
}
```

The last token is interpreted as mode if it matches `require`|`refuse`|`enable`|`disable`. Otherwise the existing direction parsing applies unchanged.

### Defaults (unchanged)

| Capability | Default | Notes |
|------------|---------|-------|
| `asn4` | `enable` | RFC 6793 — on unless explicitly disabled |
| `extended-message` | `disable` | Opt-in |
| `route-refresh` | `disable` | Opt-in |
| `graceful-restart` | `disable` | Opt-in, needs config |
| `add-path` | `disable` | Opt-in, needs config |
| `nexthop` | `disable` | Opt-in, needs config |
| `software-version` | `disable` | Opt-in |

### Backwards Mapping

| Old syntax | New equivalent | Accepted? |
|-----------|---------------|-----------|
| `asn4 true` | `asn4 enable` | Yes — `true` treated as `enable` |
| `asn4 false` | `asn4 disable` | Yes — `false` treated as `disable` |
| `extended-message enable` | same | No change |
| `extended-message true` | `extended-message enable` | Yes |
| `route-refresh;` (bare) | `route-refresh enable` | Yes — bare presence = enable |
| `add-path send/receive` | `add-path send/receive enable` | Yes — missing mode = enable |

## Key Insights

- Enforcement is post-negotiation: exchange OPENs, negotiate, then check requirements
- `buildUnsupportedCapabilityData` only handles family (Multiprotocol) capabilities — separate `buildUnsupportedCapabilityDataCodes` added for non-family codes
- `Mismatches` slice in `Negotiated` tracks which capabilities weren't agreed — enforcement leverages this

## Data Flow

### Entry Point
- Config file parsed into tree; `parseCapabilitiesFromTree()` reads capability modes
- Peer OPEN message received as wire bytes via TCP

### Transformation Path
1. Config parsing: `parseCapabilitiesFromTree()` extracts mode per capability, stores in PeerSettings
2. OPEN building: capabilities advertised based on mode (enable/require advertise; disable/refuse don't)
3. OPEN exchange: local and remote OPENs received
4. Negotiation: `capability.Negotiate()` computes intersection + Mismatches + peerCodes
5. Validation: `CheckRequiredCodes()` + `CheckRefusedCodes()` on Negotiated result
6. Enforcement: NOTIFICATION (Unsupported Capability) closes session on violation

### Boundaries Crossed
| Boundary | How |
|----------|-----|
| Config to PeerSettings | Tree parsing in `config.go` |
| PeerSettings to Session | Settings passed to session constructor |
| Session to Negotiated | `Negotiate()` called after OPEN exchange |
| Session to Wire | NOTIFICATION sent via `sendNotification()` |

### Integration Points
- `parseCapabilitiesFromTree()` — extended to parse mode values
- `PeerSettings` — `RequiredCapabilities` and `RefusedCapabilities` fields added
- `Negotiated.CheckRequiredCodes()` / `CheckRefusedCodes()` — new validation methods
- `session.go` validation — extracted `validateCapabilityModes()` helper, called in both `processOpen()` and `handleOpen()`
- `buildUnsupportedCapabilityDataCodes()` — separate function for non-Multiprotocol capability codes

## Design Decisions

### Mode pushed down for add-path
Per-family mode allows `ipv4/unicast send require;` rather than requiring all-or-nothing. Matches how family require already works at the family level.

### Refused needs peer's raw capabilities, not negotiated
Negotiation is intersection — a refused capability won't appear in negotiated set. Must check peer's OPEN directly. `peerCodes map[Code]bool` stored in Negotiated during `Negotiate()` gives access to peer's raw advertised capability codes.

### buildUnsupportedCapabilityData kept separate
Created `buildUnsupportedCapabilityDataCodes()` for non-family codes rather than generalising the existing family function. Simpler — no interface changes needed.

### ze-peer option-based control
Used `.ci` file options (`drop-capability`/`add-capability`) to control ze-peer's OPEN capabilities, instead of cap 254 in-band signaling. Simpler — no raw capability config needed in ze.

### TypeBool extended rather than YANG type changed
`asn4` kept as YANG `type boolean`. `ValidateValue(TypeBool, ...)` extended to accept `require`/`refuse` alongside `true/false/enable/disable`. This preserves the parser's `NormalizeBool()` behavior and serializer roundtrip.

### capModeTokens as single source of truth
`isCapModeToken()` derives from `capModeTokens` slice via `slices.Contains()` — no separate switch statement to maintain.

## RFC References
- RFC 5492 Section 3 — NOTIFICATION with Unsupported Capability subcode (error code 2, subcode 7)
- RFC 6793 Section 3 — ASN4 capability (code 65)
- RFC 7911 Section 4 — ADD-PATH capability (code 69)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `asn4 require;` + peer without ASN4 | Session rejected with NOTIFICATION (Unsupported Capability) |
| AC-2 | `asn4 require;` + peer with ASN4 | Session established normally |
| AC-3 | `asn4 refuse;` + peer with ASN4 | Session rejected with NOTIFICATION |
| AC-4 | `asn4 refuse;` + peer without ASN4 | Session established (ASN4 not advertised by either) |
| AC-5 | `route-refresh require;` + peer without route-refresh | Session rejected |
| AC-6 | `extended-message require;` + peer with extended-message | Session established |
| AC-7 | `add-path send/receive require;` + peer without add-path | Session rejected |
| AC-8 | `add-path { ipv4/unicast send require; }` + peer without add-path for ipv4/unicast | Session rejected (code-level only, not per-family granularity) |
| AC-9 | `add-path { ipv4/unicast send require; ipv6/unicast send; }` + peer with only ipv6/unicast add-path | Session rejected (code-level only) |
| AC-10 | `graceful-restart { mode require; restart-time 120; }` + peer without GR | Session rejected |
| AC-11 | `asn4 true;` (old syntax) | Parsed as `enable` — backwards compatible |
| AC-12 | `asn4 enable;` (new syntax) | Same as `asn4 true;` |
| AC-13 | Config with no capability block | Defaults unchanged (ASN4 enabled, others disabled) |

## What Was Implemented

- `capMode` type with `parseCapMode()` in config.go — handles enable/disable/require/refuse/true/false
- `applyCapMode()` helper populates RequiredCapabilities/RefusedCapabilities on PeerSettings
- `capModeTokens` var + `isCapModeToken()` (derives via `slices.Contains`) for trailing mode token detection
- Updated `parseCapabilitiesFromTree()` for simple caps (ASN4, extended-message, route-refresh, software-version)
- Updated `parseAddPathFromTree()` for global and per-family trailing mode tokens
- GR and nexthop block `mode` key support in `parseCapabilitiesFromTree()`
- `RequiredCapabilities` and `RefusedCapabilities` fields on PeerSettings
- `peerCodes map[Code]bool` in Negotiated — tracks peer's raw advertised capability codes
- `CheckRequiredCodes()` and `CheckRefusedCodes()` methods on Negotiated
- `buildUnsupportedCapabilityDataCodes()` for non-Multiprotocol NOTIFICATION data
- `validateCapabilityModes()` extracted helper — used in both `processOpen()` and `handleOpen()`
- `CapabilityOverride` struct + `applyCapabilityOverrides()` in ze-peer for functional test support
- `TypeBool` validation extended for require/refuse in schema.go

## Bugs Found/Fixed

- Absent opt-in capabilities parsed as `enable` because `flexString("")` returns `""` and `parseCapMode("")` defaulted to enable. Fixed by checking `v != ""` before calling parseCapMode.
- Per-family add-path test failed because missing `"capability"` block in test tree caused early return. Fixed by adding empty capability map.
- YANG `asn4` temporarily changed to `type string` — broke `TestEnableDisable` and `TestSerializeCapability`. Reverted to `type boolean` and extended `TypeBool` validator instead.
- `cmd/ze-test/peer.go` `parsePeerFlags()` didn't merge `CapabilityOverrides` from file config. Fixed.
- `dupSubExpr` lint error when adding CheckRequiredCodes static map entry with duplicated expression. Resolved with maintenance comment.
- `perFamilyHasMode` naming misleading — renamed to `perFamilyHasEnforcement` for clarity.
- `isCapModeToken` had separate switch statement from `capModeTokens` var — unified to derive from slice.

## Documentation Updates
- `docs/architecture/wire/capabilities.md` — Added "Capability Mode Enforcement" section
- `docs/architecture/config/syntax.md` — Rewrote "Capability Section" with mode vocabulary
- `docs/architecture/testing/ci-format.md` — Added OPEN behaviors, drop/add-capability docs

## Files Modified
- `internal/plugins/bgp/reactor/config.go` — capMode type, parsing, mode enforcement for all capabilities
- `internal/plugins/bgp/reactor/config_test.go` — 7 new test functions (35+ subtests)
- `internal/plugins/bgp/reactor/session.go` — validateCapabilityModes helper, enforcement in both OPEN paths
- `internal/plugins/bgp/reactor/session_test.go` — 4 new tests for session enforcement + wire encoding
- `internal/plugins/bgp/reactor/peersettings.go` — RequiredCapabilities, RefusedCapabilities fields
- `internal/plugins/bgp/capability/negotiated.go` — peerCodes map, CheckRequiredCodes, CheckRefusedCodes
- `internal/plugins/bgp/capability/negotiated_test.go` — TestCheckRequiredCodes, TestCheckRefusedCodes
- `internal/config/schema.go` — TypeBool accepts require/refuse
- `internal/config/schema_test.go` — boolean validation tests
- `internal/test/peer/peer.go` — CapabilityOverride, applyCapabilityOverrides, drop/add-capability parsing
- `cmd/ze-test/peer.go` — CapabilityOverrides merge
- `internal/plugins/bgp/schema/ze-bgp-conf.yang` — asn4 description update
- `docs/architecture/config/syntax.md` — capability mode documentation
- `docs/architecture/testing/ci-format.md` — OPEN behaviors, capability control docs
- `docs/architecture/wire/capabilities.md` — mode enforcement documentation

## Files Created
- `test/encode/cap-require-asn4.ci` — functional test: require ASN4
- `test/encode/cap-refuse-asn4.ci` — functional test: refuse ASN4

## Deviations from Plan
- `buildUnsupportedCapabilityData()` not generalised — created separate `buildUnsupportedCapabilityDataCodes()` instead (simpler)
- ze-peer control: used `.ci` file options instead of cap 254 in-band control (simpler)
- Add-path per-family require (AC-8/9): code-level enforcement only, not per-family granularity. True per-family add-path enforcement would need a separate mechanism.
