# Spec: paths-limit

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 10/11 |
| Updated | 2026-06-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/wire/capabilities.md` - capability negotiation
4. `internal/component/bgp/capability/capability.go` - capability types and parsing
5. `internal/component/bgp/capability/negotiated.go` - negotiation logic
6. `internal/component/bgp/reactor/config_capabilities.go` - config to capability mapping
7. ExaBGP reference: `src/exabgp/bgp/message/open/capability/pathslimit.py`

## Task

Two changes in one spec:

1. **Restructure add-path config.** Unify the two current add-path config locations (global `session > capability > add-path { send true; receive true; }` and peer-level `add-path` list) into a single `session > capability > add-path` block with a default `direction` and per-family overrides.

2. **Add PATHS-LIMIT capability** (draft-abraitis-idr-addpath-paths-limit). This capability extends ADD-PATH (RFC 7911) by letting a receiver advertise the maximum number of paths it wants to receive per prefix, per address family. The remote speaker's PATHS-LIMIT constrains our outgoing path count; our PATHS-LIMIT constrains theirs. ExaBGP already implements this; ze needs: wire encoding/decoding, config/YANG support, negotiation, bridge event conversion, exabgp migration, and outbound enforcement.

PATHS-LIMIT is only meaningful when ADD-PATH is also negotiated. It is a capability (OPEN message), not a path attribute (UPDATE message).

### Target config syntax (user-authorized exception to spec-no-code rule)

```
session {
    capability {
        add-path {
            direction send;                  # default for all negotiated families
            limit 5;                         # default paths-limit for all families
            family ipv4/unicast {
                direction send/receive;      # override
                limit 10;                    # per-family override
            }
            family ipv6/unicast {
                direction receive;           # override, inherits limit 5
            }
        }
    }
}
```

- `direction` on the container: default for all negotiated multiprotocol families
- `limit` on the container: default PATHS-LIMIT inherited by all families (overridden per-family)
- `family` list inside the container: per-family overrides with optional `direction`, `limit`, `mode`
- Replaces: `session > capability > add-path { send true; receive true; }` (boolean leaves removed)
- Replaces: peer-level `add-path` list (removed from peer level)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/capabilities.md` - capability negotiation
  → Constraint: capabilities implement the Capability interface (Code, Len, WriteTo)
  → Constraint: negotiation in negotiated.go, encoding caps in encoding.go
- [ ] `docs/architecture/core-design.md` - overall architecture, bridge, migration patterns

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7911.md` - ADD-PATH (prerequisite for PATHS-LIMIT)
  → Constraint: PATHS-LIMIT is only relevant when ADD-PATH is negotiated for the family
- [ ] `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` - PATHS-LIMIT draft (create if missing)
  → Constraint: Capability code 76. 5-byte entries: AFI(2)+SAFI(1)+Limit(2). Limit 1-65535; 0 means skip.

**Key insights:**
- PATHS-LIMIT is a capability (code 76), not an attribute
- Wire format: variable-length sequence of 5-byte entries (AFI 2 bytes, SAFI 1 byte, Limit 2 bytes, big-endian)
- Only emitted in OPEN if at least one family has a limit > 0
- On receive, PATHS-LIMIT entries are only accepted for families also present in peer's ADD-PATH capability
- ExaBGP config: `add-path { ipv4 unicast limit 10; }` - the `limit N` suffix is optional per family
- Negotiation is receiver-advertised: a speaker advertises how many paths it wants to receive per prefix. The remote PATHS-LIMIT constrains our send context; our PATHS-LIMIT constrains the peer's send context.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/capability/capability.go` - defines Code enum (1-73), Capability interface, parse/write for all known capabilities. CodePathsLimit (76) absent.
  → Constraint: new capability must follow same pattern as AddPath: struct + parse function + WriteTo + Code constant + switch case in parseCapability
- [ ] `internal/component/bgp/capability/negotiated.go` - Negotiate() builds Negotiated from local+remote caps. ADD-PATH negotiation at lines 253-291. No PATHS-LIMIT handling.
  → Constraint: PATHS-LIMIT negotiation must run after ADD-PATH negotiation (depends on addPath map being populated)
- [ ] `internal/component/bgp/capability/encoding.go` - EncodingCaps holds wire-encoding-affecting caps (ASN4, families, AddPathMode, ExtendedNextHop). Shared by recv/send EncodingContexts (context.go:58-69). No paths-limit field.
  → Decision: Two maps in EncodingCaps: `PathsLimitSend map[Family]uint16` (remote peer's advertised limits, constrains our send) and `PathsLimitRecv map[Family]uint16` (our advertised limits, constrains peer's send). Mirrors AddPathMode direction-aware pattern.
- [ ] `internal/component/bgp/context/context.go` - EncodingContext derives direction-specific addPath map from encoding.AddPathMode + direction (lines 85-101). Hash includes addPath and direction (lines 206-252).
  → Constraint: must derive direction-specific `pathsLimit map[family.Family]uint16` from EncodingCaps (send context uses PathsLimitSend, recv context uses PathsLimitRecv). Include in hash for deduplication. New accessor `PathsLimit(f Family) uint16` returning 0 for no limit.
- [ ] `internal/component/bgp/reactor/config_capabilities.go` - parseAddPathFromTree (line 289) currently reads from two sources: capMap["add-path"] for global send/receive and peerTree["add-path"] for per-family overrides. No paths-limit parsing.
  → Constraint: must be rewritten to read from unified `session > capability > add-path` block: default direction + family list with direction/limit/mode
- [ ] `internal/component/bgp/reactor/peersettings.go` - PeerSettings struct holds Capabilities slice. No PathsLimit field needed (stored as Capability in the slice).
- [ ] `internal/component/bgp/format/decode.go` - DecodedNegotiated struct (line 366) holds AddPathSend/AddPathReceive. NegotiatedToDecoded (line 386) converts from capability.Negotiated. No paths-limit fields.
  → Constraint: must add PathsLimit field to DecodedNegotiated and populate from negotiated caps
- [ ] `internal/component/bgp/format/json.go` - JSON encoder for negotiated events. Encodes add-path (line 223-245). No paths-limit.
  → Constraint: must encode paths-limit in negotiated JSON output
- [ ] `internal/component/bgp/cli/decode_open.go` - capabilityToZeJSON (line 72) handles AddPath (line 84) but not PathsLimit. User-visible OPEN decode output.
  → Constraint: must add PathsLimit case for `ze bgp decode` to display paths-limit capability
- [ ] `internal/exabgp/bridge/bridge_event.go` - convertNegotiated converts ze negotiated caps to ExaBGP JSON. Converts add-path key. No paths-limit.
  → Constraint: must convert paths-limit in negotiated events for ExaBGP compatibility
- [ ] `internal/exabgp/migration/migrate.go` - migrateCapability (line 538) reads capability-level add-path. But ExaBGP per-family add-path is at neighbor level (exabgp.yang:233, freeform), not capability level. copyContainers (line 653-682) omits neighbor-level add-path. limit N suffix is per-family in the freeform block.
  → Constraint: need dedicated neighbor-level add-path conversion to unified ze syntax, not just migrateCapability extension
- [ ] `internal/exabgp/migration/migrate_serialize.go` - serializer (line 234-256) does not emit nested family direction/limit format.
  → Constraint: must update serializer to emit the new `add-path { direction ...; family ... { ... } }` structure
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - Two separate add-path configs: (1) capability container at line 653 with boolean send/receive, (2) peer-level list at line 693 with direction/mode per family. No limit leaf anywhere.
  → Decision: Unify into single `session > capability > add-path` container: replace send/receive booleans with direction enum; add family list inside with direction/limit/mode; remove peer-level add-path list. User confirmed this design.
- [ ] `internal/component/bgp/rib/commit.go` - CommitService builds UPDATE messages. Uses addPathFor(). No path counting/limiting. Used by named SendRoutes (reactor_api_batch.go:586-623).
  → Constraint: one of multiple outbound paths needing enforcement
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - sendInitialRoutes sends static routes directly (line 76-142). Does not go through CommitService.
  → Constraint: separate enforcement point needed here
- [ ] `internal/component/bgp/reactor/forward_rs.go` - reactorForwardRS writes raw/parsed bodies directly for RS fast-path (line 53-61). Bypasses RIB.
  → Constraint: RS fast-path may need enforcement or should not advertise PATHS-LIMIT

**Behavior to preserve:**
- ADD-PATH negotiation logic (PATHS-LIMIT is additive, must not change ADD-PATH behavior)
- ADD-PATH wire encoding (capability code 69, 4-byte entries)
- Existing capability wire encoding format for all other capabilities
- Config parsing for all existing non-add-path capabilities
- Bridge event format for non-paths-limit capabilities
- Migration of all existing ExaBGP capabilities (non-add-path ones unchanged)
- Functional equivalence: any config expressible in the old two-location add-path schema must be expressible in the new unified schema

**Behavior to change:**
- Restructure add-path YANG: remove boolean send/receive from capability container, remove peer-level add-path list, replace with unified container having direction enum + family list
- Rewrite parseAddPathFromTree to read from unified location
- Add PATHS-LIMIT capability code 76 to the capability system
- Add PATHS-LIMIT to OPEN negotiation (coupled with ADD-PATH)
- Add paths-limit to negotiated event JSON
- Add paths-limit to bridge event conversion
- Add per-family limit leaf to new family list inside add-path
- Update ExaBGP migration to produce new unified config format
- Enforce path count limits in RIB outgoing path selection

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config: YANG add-path list entry with optional `limit` leaf
- Wire: OPEN message capability TLV with code 76
- Bridge: ExaBGP add-path config with `limit N` suffix

### Transformation Path
1. Config tree resolution: `parsePeerFromTree` -> `parseCapabilitiesFromTree` -> `parseAddPathFromTree` (extended to parse limit) -> builds `PathsLimit` capability
2. OPEN encoding: `PathsLimit.WriteTo()` encodes capability TLV into OPEN
3. OPEN parsing: `parseCapability()` dispatches code 76 to `parsePathsLimit()` -> returns `PathsLimit` struct
4. Negotiation: `Negotiate()` processes PATHS-LIMIT after ADD-PATH, stores limits in `EncodingCaps.PathsLimit`
5. Enforcement: all outbound paths check `PathsLimit` map: CommitService (rib/commit.go), initial sync (peer_initial_sync.go), RS fast-path (forward_rs.go)
6. Events: DecodedNegotiated includes paths-limit (format/decode.go); JSON encoder emits it (format/json.go); CLI decode shows it (cli/decode_open.go); bridge converts to ExaBGP format
7. Migration: dedicated neighbor-level add-path conversion parses ExaBGP freeform add-path with `limit N` suffix

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Capability | parseAddPathFromTree builds PathsLimit and appends to PeerSettings.Capabilities | [ ] |
| Wire -> Capability | parsePathsLimit returns PathsLimit struct from OPEN bytes | [ ] |
| Capability -> Negotiated | Negotiate() cross-references PathsLimit with AddPath | [ ] |
| Negotiated -> EncodingCaps | buildSubComponents copies PathsLimit map to EncodingCaps | [ ] |
| EncodingCaps -> Outbound | CommitService, initial sync, and RS fast-path read PathsLimit to limit outgoing paths | [ ] |
| Negotiated -> DecodedNegotiated | NegotiatedToDecoded (format/decode.go) populates PathsLimit field | [ ] |
| DecodedNegotiated -> JSON | json.go encodes paths-limit in negotiated event | [ ] |
| Capability -> CLI decode | capabilityToZeJSON (cli/decode_open.go) formats PathsLimit for display | [ ] |
| Ze event -> ExaBGP JSON | convertNegotiated includes paths-limit | [ ] |
| ExaBGP config -> Ze config | dedicated neighbor-level add-path conversion (not just migrateCapability) | [ ] |

### Integration Points
- `capability.AddPath` - PATHS-LIMIT depends on ADD-PATH being negotiated for the same family
- `capability.Negotiate()` - must run PATHS-LIMIT after ADD-PATH
- `EncodingCaps` - new `PathsLimit` field for outgoing enforcement
- `CommitService` - enforcement point for path count limits
- `bridge_event.go:convertNegotiated` - ExaBGP JSON conversion
- `migration/migrate.go` - dedicated neighbor-level add-path conversion (ExaBGP freeform add-path is at neighbor level, not capability level)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | PATHS-LIMIT capability code is 76 (0x4C) | draft-abraitis-idr-addpath-paths-limit-04, exabgp pathslimit.py | Wrong code would break interop | grep exabgp code for 0x4C/76 | confirmed |
| A-2 | Wire format is AFI(2)+SAFI(1)+Limit(2) per entry | exabgp pathslimit.py extract_capability_bytes | Wrong format breaks wire interop | exabgp source + draft | confirmed |
| A-3 | PATHS-LIMIT is only meaningful with ADD-PATH | draft + exabgp negotiated.py lines 164-186 | Advertising without ADD-PATH wastes bytes | draft text | confirmed |
| A-4 | Limit 0 means "skip/ignore this entry" | exabgp pathslimit.py line 48 (skip limit==0) | Accepting 0 could mean "no paths" | exabgp source | confirmed |
| A-5 | ExaBGP config uses `limit N` suffix in add-path block | exabgp family.py ParseAddPath._parse_addpath_family | Wrong migration parsing | exabgp source | confirmed |
| A-6 | RIB CommitService is the correct enforcement point | rib/commit.go builds outgoing UPDATEs | Enforcing in wrong place misses paths or over-sends | code reading | unvalidated |
| A-7 | EncodingCaps with direction-aware maps (PathsLimitSend/Recv) is correct, mirroring AddPathMode pattern | encoding.go holds wire-affecting caps; context.go derives direction-specific state | Wrong placement = unavailable at enforcement or wrong direction semantics | code reading of context.go:85-101 | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Path counting in RIB may need per-prefix tracking | Testing with multiple paths per prefix | Use existing RIB data structures to count current paths per prefix before sending |
| R-2 | Route Server fast-path bypasses RIB enforcement | RSFastPath peers skip normal commit path | Either enforce in forward_rs or suppress PATHS-LIMIT capability for RSFastPath peers |
| R-4 | Initial static route send bypasses CommitService | peer_initial_sync.go sends directly | Add enforcement in sendInitialRoutes before UPDATE construction |
| R-5 | Per-prefix path counting requires tracking across path IDs and AS_PATH groups | Grouping splits by AS_PATH (rib/grouping.go:161-167) | Count before grouping; track by canonical prefix key regardless of path ID or attribute group |
| R-3 | ExaBGP has add-path at two levels: capability (exabgp.yang:90-94, direction) and neighbor freeform (exabgp.yang:232-233, per-family selection + limit) | Migration tests with capability-only, neighbor-only, and combined | Dedicated conversion reads both levels: direction from capability, families+limits from neighbor-level freeform |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| OPEN bytes with code 76 | -> | parsePathsLimit in capability.go | TestParsePathsLimit |
| Unified add-path config with default direction | -> | parseAddPathFromTree producing AddPath capability for all families | TestParseAddPathUnifiedDefault |
| Unified add-path config with per-family limit | -> | parseAddPathFromTree producing PathsLimit capability | TestParseAddPathWithPathsLimit |
| Negotiate with PathsLimit caps | -> | Negotiate() stores limits in EncodingCaps | TestNegotiatePathsLimit |
| ExaBGP add-path with limit | -> | neighbor-level add-path conversion preserves limit | TestMigrateAddPathWithLimit |
| Ze negotiated event | -> | convertNegotiated includes paths-limit | TestConvertNegotiatedPathsLimit |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| | **Config restructuring** | |
| AC-1 | `add-path { direction send; }` (no family overrides) | ADD-PATH send advertised for all negotiated multiprotocol families |
| AC-2 | `add-path { direction send; family ipv4/unicast { direction send/receive; } }` | ipv4/unicast gets send/receive; all other families get send |
| AC-3 | `add-path { direction send; family ipv6/unicast { direction receive; } }` | ipv6/unicast overridden to receive; others get send |
| AC-4 | `add-path { family ipv4/unicast { direction send; } }` (no default direction) | Only ipv4/unicast gets ADD-PATH; other families unaffected |
| AC-5 | Old peer-level `add-path` list syntax | Rejected by YANG validation (removed from schema) |
| AC-6 | Old `add-path { send true; receive true; }` syntax | Rejected by YANG validation (boolean leaves removed) |
| | **PATHS-LIMIT wire encoding** | |
| AC-7 | OPEN message contains capability code 76 with entries | Parsed into PathsLimit struct with correct AFI/SAFI/Limit values |
| AC-8 | PathsLimit capability with families | WriteTo produces correct wire bytes (code 76 + 5-byte entries) |
| AC-9 | Round-trip: encode then parse PathsLimit | Identical struct after decode |
| AC-10 | Limit value 0 in received OPEN | Entry skipped/ignored during parsing |
| AC-11 | PathsLimit capability with zero entries | Capability not advertised in OPEN |
| AC-11b | Duplicate AFI/SAFI entries in received PATHS-LIMIT | First entry wins, duplicates silently ignored (per draft) |
| | **PATHS-LIMIT negotiation** | |
| AC-12 | Both peers advertise PathsLimit and AddPath for same families | Negotiated.EncodingCaps.PathsLimit contains per-family limits |
| AC-13 | Peer advertises PathsLimit for family without ADD-PATH | Family excluded from negotiated PathsLimit |
| AC-14 | Only one peer advertises PathsLimit | No paths-limit negotiated for the non-advertising direction |
| | **PATHS-LIMIT config** | |
| AC-15 | `add-path { family ipv4/unicast { direction send; limit 10; } }` | Both ADD-PATH and PathsLimit capabilities in OPEN for ipv4/unicast |
| AC-16 | `add-path { family ipv4/unicast { direction send; } }` (no limit) | Only ADD-PATH, no PathsLimit capability |
| AC-17 | Limit value boundary: 1 | Minimum valid limit accepted by YANG |
| AC-18 | Limit value boundary: 65535 | Maximum valid limit accepted by YANG |
| AC-19 | Limit value 0 in config | Rejected by YANG range constraint |
| | **Migration and bridge** | |
| AC-20 | ExaBGP `capability { add-path { send true; receive true; } }` + neighbor `add-path { ipv4 unicast limit 10; }` migrated | Ze config: `add-path { direction send/receive; family ipv4/unicast { limit 10; } }`. Direction derived from capability block. |
| AC-21 | ExaBGP `capability { add-path { send true; } }` + neighbor `add-path { ipv4 unicast; }` migrated (send-only, no limit) | Ze config: `add-path { direction send; family ipv4/unicast { } }` |
| AC-21b | ExaBGP `capability { add-path { receive true; } }` + neighbor `add-path { ipv6 unicast; }` migrated (receive-only) | Ze config: `add-path { direction receive; family ipv6/unicast { } }` |
| AC-21c | ExaBGP neighbor `add-path { ipv4 unicast; }` with no capability block | Ze config derives direction from ExaBGP defaults (send/receive) |
| AC-22 | Ze negotiated event JSON | Contains "paths-limit" with direction-aware structure matching add-path pattern: `"paths-limit": {"send": {"ipv4/unicast": 10}, "receive": {"ipv6/unicast": 20}}`. Send = limits we enforce (from remote), receive = limits peer enforces (from us). |
| AC-23 | Bridge converts negotiated event | ExaBGP JSON includes `"paths_limit": {"send": {"ipv4 unicast": 10}, "receive": {"ipv6 unicast": 20}}`. Families use space separator (ExaBGP convention), keys use underscore. |
| | **Mode enforcement** | |
| AC-24 | `add-path { family ipv4/unicast { direction send; mode require; } }` | Session rejected if peer does not negotiate ADD-PATH for ipv4/unicast. Requires new per-family required/refused validation in session_validation.go (current RequiredCapabilities/RefusedCapabilities are code-level only, not per-family). |
| AC-25 | `add-path { family ipv4/unicast { direction send; mode refuse; } }` | Session rejected if peer advertises ADD-PATH for ipv4/unicast |
| AC-26 | Default direction + per-family mode: `add-path { direction send; family ipv4/unicast { mode require; } }` | Require applies to ipv4/unicast; other families get enable (default) |
| | **Outbound enforcement** | |
| AC-27 | CommitService outgoing path selection with paths-limit negotiated | Paths beyond limit per prefix silently dropped (canonical prefix counting before grouping, across path ID and AS_PATH) |
| AC-28 | Initial static route send (peer_initial_sync.go) with paths-limit | Paths beyond limit per prefix dropped during initial sync |
| AC-29 | RS fast-path peers | Either enforce limit in forward_rs or suppress PATHS-LIMIT capability advertisement for RSFastPath peers |
| | **CLI decode** | |
| AC-30 | `ze bgp decode` on OPEN with PathsLimit capability | Output shows paths-limit with per-family limits |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures add-path with limit 10 for ipv4/unicast | YANG -> config tree -> parseAddPathFromTree -> PeerSettings.Capabilities includes PathsLimit -> OPEN message | TestConfigPathsLimitEndToEnd |
| 2 | Receives OPEN with PathsLimit from peer | wire bytes -> parseCapability(76) -> PathsLimit struct -> Negotiate() -> EncodingCaps.PathsLimit | TestReceivePathsLimitEndToEnd |
| 3 | Migrates ExaBGP config with add-path limit | exabgp.conf -> neighbor-level add-path conversion (direction from capability block, families+limits from neighbor freeform) -> ze unified add-path config | TestMigratePathsLimitEndToEnd |
| 4 | Bridge receives negotiated event with paths-limit | ze JSON event -> convertNegotiated -> ExaBGP JSON with paths_limit | TestBridgePathsLimitEvent |
| 5 | RIB enforces path limit on outgoing updates | routes announced -> CommitService checks PathsLimit -> excess paths dropped | TestRIBPathsLimitEnforcement |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestParsePathsLimit | internal/component/bgp/capability/capability_test.go | Parse 5-byte entries from wire bytes | |
| TestParsePathsLimitEmpty | internal/component/bgp/capability/capability_test.go | Empty data produces empty PathsLimit | |
| TestParsePathsLimitShortRead | internal/component/bgp/capability/capability_test.go | Truncated data returns ErrShortRead | |
| TestParsePathsLimitSkipZero | internal/component/bgp/capability/capability_test.go | Entries with limit 0 are skipped | |
| TestPathsLimitWriteTo | internal/component/bgp/capability/capability_test.go | WriteTo produces correct wire bytes | |
| TestPathsLimitRoundTrip | internal/component/bgp/capability/capability_test.go | Encode then decode yields same struct | |
| TestPathsLimitLen | internal/component/bgp/capability/capability_test.go | Len returns 2 + 5*N | |
| TestPathsLimitCode | internal/component/bgp/capability/capability_test.go | Code() returns 76 | |
| TestPathsLimitConfigValues | internal/component/bgp/capability/capability_test.go | ConfigValues returns scoped keys | |
| TestNegotiatePathsLimit | internal/component/bgp/capability/negotiated_test.go | Both peers advertise: limits stored | |
| TestNegotiatePathsLimitOneSided | internal/component/bgp/capability/negotiated_test.go | Only one peer: no limits negotiated for missing direction | |
| TestNegotiatePathsLimitNoAddPath | internal/component/bgp/capability/negotiated_test.go | PathsLimit without AddPath: family excluded | |
| TestNegotiatePathsLimitPartialAddPath | internal/component/bgp/capability/negotiated_test.go | PathsLimit for family not in AddPath: that family excluded | |
| TestParseAddPathUnifiedDefault | internal/component/bgp/reactor/config_capabilities_test.go | Unified config with default direction applies to all families | |
| TestParseAddPathUnifiedPerFamily | internal/component/bgp/reactor/config_capabilities_test.go | Per-family entries in unified config produce correct AddPathFamily entries | |
| TestParseAddPathUnifiedOverride | internal/component/bgp/reactor/config_capabilities_test.go | Per-family direction overrides default direction | |
| TestParseAddPathWithLimit | internal/component/bgp/reactor/config_capabilities_test.go | Per-family entry with limit produces PathsLimit capability | |
| TestParseAddPathWithoutLimit | internal/component/bgp/reactor/config_capabilities_test.go | Per-family entry without limit: no PathsLimit capability | |
| TestParseAddPathNoDefault | internal/component/bgp/reactor/config_capabilities_test.go | No default direction, only per-family: only listed families get ADD-PATH | |
| TestMigrateCapabilityPathsLimit | internal/exabgp/migration/migrate_test.go | ExaBGP add-path with limit migrated correctly | |
| TestMigrateCapabilityPathsLimitNoLimit | internal/exabgp/migration/migrate_test.go | ExaBGP add-path without limit: no limit in output | |
| TestConvertNegotiatedPathsLimit | internal/exabgp/bridge/bridge_test.go | Negotiated event with paths-limit converted to ExaBGP JSON | |
| TestParseAddPathModeRequire | internal/component/bgp/reactor/config_capabilities_test.go | Per-family mode require produces RequiredAddPathFamilies entry | |
| TestParseAddPathModeRefuse | internal/component/bgp/reactor/config_capabilities_test.go | Per-family mode refuse produces RefusedAddPathFamilies entry | |
| TestParseAddPathModePrecedence | internal/component/bgp/reactor/config_capabilities_test.go | Default direction + per-family mode override: correct per-family behavior | |
| TestValidateAddPathFamilyRequired | internal/component/bgp/reactor/session_validation_test.go | Per-family required ADD-PATH rejects session when family not negotiated | |
| TestValidateAddPathFamilyRefused | internal/component/bgp/reactor/session_validation_test.go | Per-family refused ADD-PATH rejects session when family present in peer OPEN | |
| TestNegotiatedToDecodedPathsLimit | internal/component/bgp/format/decode_test.go | DecodedNegotiated includes PathsLimit from negotiated caps | |
| TestNegotiatedJSONPathsLimit | internal/component/bgp/format/json_test.go | JSON negotiated event includes paths-limit with send/receive family lists | |
| TestDecodeOpenPathsLimit | internal/component/bgp/cli/decode_open_test.go | capabilityToZeJSON formats PathsLimit with per-family limits | |
| TestCommitServicePathsLimit | internal/component/bgp/rib/commit_test.go | Excess paths per prefix dropped when PathsLimit negotiated | |
| TestInitialSyncPathsLimit | internal/component/bgp/reactor/peer_initial_sync_test.go | Static route send respects PathsLimit | |
| TestRSFastPathPathsLimit | internal/component/bgp/reactor/forward_rs_test.go | RS fast-path enforces or suppresses PATHS-LIMIT | |
| TestEncodingContextPathsLimit | internal/component/bgp/context/context_test.go | Direction-specific pathsLimit derived from EncodingCaps | |
| TestEncodingContextHashIncludesPathsLimit | internal/component/bgp/context/context_test.go | Hash changes when pathsLimit differs | |
| TestParsePathsLimitDuplicateFirstWins | internal/component/bgp/capability/capability_test.go | Duplicate AFI/SAFI in wire: first entry kept, later ignored | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Limit (wire) | 0-65535 | 65535 | N/A (uint16) | N/A (uint16) |
| Limit (config) | 1-65535 | 65535 | 0 (rejected) | 65536 (rejected by YANG range) |
| Entry count | 0-50 ((255-2)/5) | 50 entries | N/A | buildOptionalParams skips capabilities with Len()>255 (session_negotiate.go:213); Len() includes 2-byte TLV header |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| decode-paths-limit | test/decode/bgp-paths-limit.ci | User decodes OPEN with paths-limit capability | |
| encode-paths-limit | test/encode/paths-limit.ci | User configures add-path with limit, OPEN contains capability | |
| migrate-paths-limit | test/exabgp-compat/encoding/conf-paths-limit.ci | ExaBGP config with paths-limit migrates correctly | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| paths-limit-exabgp | test/interop/scenarios/ | ExaBGP | Ze and ExaBGP negotiate PATHS-LIMIT correctly | |

### Future (if deferring any tests)
- RIB enforcement integration test: may require a multi-path test harness to verify path counting (requires user approval to defer)

## Files to Modify
- `internal/component/bgp/capability/capability.go` - add CodePathsLimit const, PathsLimit struct, parsePathsLimit, switch case, Code.String() case, ConfigValues
- `internal/component/bgp/capability/negotiated.go` - add PATHS-LIMIT negotiation after ADD-PATH in Negotiate(); add CodePathsLimit to CheckRequiredCodes map
- `internal/component/bgp/capability/encoding.go` - add PathsLimit field to EncodingCaps
- `internal/component/bgp/reactor/config_capabilities.go` - rewrite parseAddPathFromTree: unified source (capability add-path block with default direction + family list with direction/limit/mode)
- `internal/component/bgp/format/decode.go` - add PathsLimit field to DecodedNegotiated, populate in NegotiatedToDecoded
- `internal/component/bgp/format/json.go` - encode paths-limit in negotiated JSON
- `internal/component/bgp/cli/decode_open.go` - add PathsLimit case to capabilityToZeJSON
- `internal/exabgp/bridge/bridge_event.go` - add paths-limit to convertNegotiated
- `internal/exabgp/migration/migrate.go` - add dedicated neighbor-level add-path conversion to unified ze format, handle limit suffix
- `internal/exabgp/migration/migrate_serialize.go` - emit nested family direction/limit structure
- `internal/component/bgp/yang/ze-bgp-conf.yang` - restructure add-path: replace boolean send/receive with direction enum on container, add family list inside with direction/limit/mode, remove peer-level add-path list
- `internal/component/bgp/rib/commit.go` - enforce paths-limit in outgoing UPDATE construction
- `internal/component/bgp/reactor/peer_initial_sync.go` - enforce paths-limit during initial static route send
- `internal/component/bgp/reactor/forward_rs.go` - either enforce paths-limit or suppress PATHS-LIMIT capability for RSFastPath peers
- `internal/component/bgp/context/context.go` - derive direction-specific pathsLimit map, new PathsLimit accessor, include in hash
- `internal/component/bgp/reactor/peersettings.go` - add RequiredAddPathFamilies/RefusedAddPathFamilies fields for per-family mode enforcement
- `internal/component/bgp/reactor/session_validation.go` - add per-family ADD-PATH required/refused validation (current validateCapabilityModes is code-level only)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | `internal/component/bgp/yang/ze-bgp-conf.yang` - restructure add-path container: direction enum + family list with direction/limit/mode; remove peer-level add-path list |
| YANG validation constraints | [ ] | limit leaf: `type uint16 { range "1..65535"; }` (range nested under type per ze-bgp-conf.yang convention). direction leaf: enumeration send/receive/send-receive |
| YANG custom validators | [ ] | Not needed - range constraint is sufficient |
| CLI commands/flags | [ ] | No new CLI commands |
| CLI grammar (action before identifier) | [ ] | N/A |
| Editor autocomplete | [ ] | Automatic for YANG numeric leaf |
| Functional test for new RPC/API | [ ] | test/decode/bgp-paths-limit.ci, test/encode/paths-limit.ci |
| Pipe completeness | [ ] | N/A |
| Env var registration | [ ] | N/A |
| Doctor check for runtime dependencies | [ ] | N/A |
| Prometheus counters/metrics | [ ] | Not for initial implementation |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` - add paths-limit to capabilities list |
| 2 | Config syntax changed? | [ ] | `docs/guide/add-path.md` (lines 8-28, 66-72 show old syntax), `docs/architecture/config/syntax.md` (lines 507-560 show add-path examples) |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/capabilities.md` - add PATHS-LIMIT capability |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` - create |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` - add PATHS-LIMIT to capability support matrix |
| 12 | Internal architecture changed? | [ ] | N/A |
| 13 | Route metadata keys added/changed? | [ ] | N/A |
| 14 | Prometheus counters added/changed? | [ ] | N/A |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] | Capability code 76 registered |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Check during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | `docs/guide/add-path.md` and `docs/architecture/config/syntax.md` show old add-path syntax that will become stale |

## Files to Create
- `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` - RFC summary for the draft
- `test/decode/bgp-paths-limit.ci` - functional test for decoding OPEN with paths-limit
- `test/encode/paths-limit.ci` - functional test for encoding paths-limit config
- `test/exabgp-compat/encoding/conf-paths-limit.ci` - migration functional test
- `test/exabgp-compat/etc/conf-paths-limit.conf` - ExaBGP config fixture (matches conf-addpath.conf pattern)
- `test/interop/scenarios/paths-limit-exabgp/` - interop test directory with scenario files

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register capability code 76, create PathsLimit struct skeleton, wire into parseCapability switch
   - Tests: TestPathsLimitCode, TestParsePathsLimit (initially fails with stub)
   - Files: capability.go (add const + struct + parse function + switch case)
   - Verify: code 76 dispatches to parsePathsLimit; test fails because parse is a stub

2. **Phase: Wire encoding/decoding** -- implement PathsLimit parse/write/len/roundtrip
   - Tests: TestParsePathsLimit, TestPathsLimitWriteTo, TestPathsLimitRoundTrip, TestPathsLimitLen, TestParsePathsLimitShortRead, TestParsePathsLimitSkipZero, TestPathsLimitConfigValues
   - Files: capability.go
   - Verify: all wire encoding tests pass

3. **Phase: Negotiation and context** -- add PATHS-LIMIT to Negotiate(), EncodingCaps, and EncodingContext
   - Tests: TestNegotiatePathsLimit, TestNegotiatePathsLimitOneSided, TestNegotiatePathsLimitNoAddPath, TestNegotiatePathsLimitPartialAddPath, TestEncodingContextPathsLimit, TestEncodingContextHashIncludesPathsLimit
   - Files: negotiated.go (negotiation + CheckRequiredCodes), encoding.go (PathsLimitSend/PathsLimitRecv maps), context/context.go (direction-specific derivation + hash + accessor)
   - Verify: negotiation stores direction-aware limits; EncodingContext derives correct per-direction map; hash differs when limits differ

4. **Phase: Config restructuring** -- unify add-path YANG and config parsing
   - Tests: TestParseAddPathUnifiedDefault, TestParseAddPathUnifiedPerFamily, TestParseAddPathUnifiedOverride, TestParseAddPathWithLimit, TestParseAddPathWithoutLimit
   - Files: ze-bgp-conf.yang (restructure add-path container), config_capabilities.go (rewrite parseAddPathFromTree)
   - Verify: unified config produces correct AddPath + PathsLimit capabilities; old boolean/list syntax rejected by YANG

5. **Phase: Events, decode, and bridge** -- add paths-limit to negotiated event format, CLI decode, and bridge conversion
   - Tests: TestNegotiatedToDecodedPathsLimit, TestConvertNegotiatedPathsLimit, TestDecodeOpenPathsLimit
   - Files: format/decode.go (DecodedNegotiated + NegotiatedToDecoded), format/json.go (JSON encoding), cli/decode_open.go (capabilityToZeJSON), bridge_event.go
   - Verify: negotiated events include paths-limit in JSON; `ze bgp decode` shows paths-limit; bridge converts correctly

6. **Phase: ExaBGP migration** -- dedicated neighbor-level add-path conversion to unified ze format
   - Tests: TestMigrateAddPathUnified, TestMigrateAddPathWithLimit, TestMigrateAddPathNoLimit
   - Files: migrate.go (neighbor-level add-path conversion), migrate_serialize.go (emit unified structure)
   - Verify: ExaBGP `add-path { ipv4 unicast limit 10; }` produces unified ze config; round-trip serialize correct

7. **Phase: Outbound enforcement** -- enforce path count limits across all outbound paths
   - Tests: TestCommitServicePathsLimit, TestInitialSyncPathsLimit, TestRSFastPathPathsLimit
   - Files: rib/commit.go, reactor/peer_initial_sync.go, reactor/forward_rs.go
   - Verify: canonical per-prefix path counting (across path ID, AS_PATH, attribute groups) before grouping; excess paths dropped in CommitService, initial sync, and RS fast-path (or PATHS-LIMIT suppressed for RS fast-path)

8. **Functional tests** -- create .ci files for decode, encode, migration
9. **RFC refs** -- create draft summary, add RFC comments to enforcing code
10. **Full verification** -- `make ze-verify`
11. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC (1-30, 11b, 21b, 21c) has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path. ExaBGP PathsLimit has everything ze's implementation has. |
| Correctness | Wire bytes match draft format exactly. Negotiation cross-references AddPath correctly. Limit 0 skipped. |
| Naming | Capability const: CodePathsLimit. Struct: PathsLimit. Config key: "limit". JSON key: "paths-limit" / "paths_limit" (ExaBGP). |
| Data flow | PathsLimit only produced when AddPath also present. Enforcement in RIB outgoing only. |
| CLI grammar | N/A (no CLI commands) |
| Doctor checks | N/A |
| YANG validation | limit leaf has `range "1..65535"` constraint |
| Prometheus counters | N/A for initial implementation |
| Rule: no-layering | No old code to replace |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| CodePathsLimit = 76 in capability.go | grep "CodePathsLimit.*76" capability.go |
| PathsLimit struct with Code/Len/WriteTo | grep "func.*PathsLimit.*Code\|Len\|WriteTo" capability.go |
| parsePathsLimit function | grep "func parsePathsLimit" capability.go |
| Switch case for code 76 in parseCapability | grep "CodePathsLimit" capability.go |
| Code.String() returns "PATHS-LIMIT(76)" | grep "PATHS-LIMIT" capability.go |
| CodePathsLimit in CheckRequiredCodes map | grep "CodePathsLimit" negotiated.go |
| PathsLimitSend and PathsLimitRecv in EncodingCaps | grep "PathsLimit" encoding.go |
| PathsLimit accessor in EncodingContext | grep "func.*EncodingContext.*PathsLimit" context/context.go |
| pathsLimit in EncodingContext hash | grep "pathsLimit" context/context.go |
| PATHS-LIMIT negotiation in Negotiate | grep "PathsLimit\|pathsLimit" negotiated.go |
| Unified add-path container with direction in YANG | grep "direction" ze-bgp-conf.yang inside add-path container |
| Family list inside add-path container in YANG | grep "list family" ze-bgp-conf.yang inside add-path |
| limit leaf in family list in YANG | grep "limit" ze-bgp-conf.yang inside add-path family |
| Peer-level add-path list removed from YANG | grep confirms no `list add-path` outside capability block |
| parseAddPathFromTree reads unified config | grep "family\|direction\|limit" config_capabilities.go |
| PathsLimit field in DecodedNegotiated | grep "PathsLimit" format/decode.go |
| paths-limit in JSON encoder | grep "paths-limit" format/json.go |
| PathsLimit case in capabilityToZeJSON | grep "PathsLimit" cli/decode_open.go |
| paths-limit in bridge conversion | grep "paths.limit" bridge_event.go |
| Neighbor-level add-path migration | grep "add-path\|limit" migrate.go (new conversion function) |
| Serializer emits unified add-path | grep "family\|limit" migrate_serialize.go |
| Enforcement in CommitService | grep "PathsLimit\|pathsLimit" rib/commit.go |
| Enforcement in initial sync | grep "PathsLimit\|pathsLimit" reactor/peer_initial_sync.go |
| RS fast-path enforcement or suppression | grep "PathsLimit\|RSFastPath" reactor/forward_rs.go or reactor/peersettings.go |
| Draft RFC summary | ls rfc/short/draft-abraitis-idr-addpath-paths-limit.md |
| Functional tests exist | ls test/decode/bgp-paths-limit.ci test/encode/paths-limit.ci |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Wire: check data length is multiple of 5 before parsing. Config: YANG range 1..65535 enforces bounds. |
| Resource exhaustion | Max 50 entries per capability ((255-2 header)/5 per entry). Limit map bounded by number of families. No amplification risk. |
| Denial of service | Malformed PathsLimit in OPEN: return ErrShortRead, session rejected cleanly. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Unify add-path config into single `session > capability > add-path` block | Keep two locations (global + peer-level list) | Current split is confusing: two places to configure the same capability. Unified block has default direction + per-family overrides + limit, all in one place. User requested this design. Breaking change to config schema, but cleaner long-term. |
| Direction enum (send/receive/send-receive) replaces boolean send/receive | Keep booleans | Enum is more explicit, matches the per-family pattern that already existed, avoids the awkward `send true; receive true;` for both directions. |
| Direction-aware PathsLimitSend/Recv maps in EncodingCaps + direction-specific derivation in EncodingContext | Single PathsLimit map; SessionCaps | PathsLimit affects wire behavior (outgoing path count). EncodingCaps is shared; EncodingContext derives per-direction state (same pattern as AddPathMode). Send context needs remote's limits; recv context needs ours. |
| Skip entries with limit 0 during parsing | Store 0 as "unlimited" | Matches ExaBGP behavior and draft semantics. 0 = ignore, not "no paths". |
| Receiver-advertised semantics: remote PATHS-LIMIT constrains our send context; our PATHS-LIMIT constrains peer's send context | Symmetric (minimum of both) | Draft section 3: "the maximum paths limit the receiver wants to receive from its peer." Matches ADD-PATH's direction-aware negotiation (negotiated.go:271-276, context.go:85-95). |

## Known Limitations
- No Prometheus counters for paths-limit enforcement (can be added in a follow-up)

## RFC Documentation

Add `// draft-abraitis-idr-addpath-paths-limit Section N: "<quoted requirement>"` above enforcing code.
MUST document: capability code, wire format, negotiation rules, limit semantics, enforcement behavior.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated]

### Deviations from Plan
- [Differences from original plan]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| PATHS-LIMIT capability parsed from wire | unit test | TestParsePathsLimit |
| PATHS-LIMIT capability encoded to wire | unit test | TestPathsLimitWriteTo |
| Negotiation with ADD-PATH cross-reference | unit test | TestNegotiatePathsLimit |
| Config produces PathsLimit capability | unit test | TestParseAddPathWithLimit |
| Unified config default + override | unit test | TestParseAddPathUnifiedOverride |
| Mode require/refuse enforcement | unit test | TestParseAddPathModeRequire |
| ExaBGP migration handles limit | unit test | TestMigrateAddPathWithLimit |
| Bridge event conversion | unit test | TestConvertNegotiatedPathsLimit |
| DecodedNegotiated includes PathsLimit | unit test | TestNegotiatedToDecodedPathsLimit |
| CLI decode shows PathsLimit | unit test | TestDecodeOpenPathsLimit |
| CommitService enforces path count | unit test | TestCommitServicePathsLimit |
| Initial sync enforces path count | unit test | TestInitialSyncPathsLimit |
| RS fast-path handled | unit test | TestRSFastPathPathsLimit |
| Interop with ExaBGP | interop test | paths-limit-exabgp scenario |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [short bullet per fix]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] All ACs demonstrated (AC-1 through AC-30, including AC-11b, AC-21b, AC-21c)
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-paths-limit.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-paths-limit.md`
