# Claude Continuation State

**Last Updated:** 2026-01-02

---

## TDD CHECKPOINT (READ BEFORE ANY IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  BEFORE writing ANY implementation code, I MUST:                │
│                                                                 │
│  1. Write a unit test that captures the expected behavior       │
│  2. Run the test → SEE IT FAIL                                  │
│  3. Paste the failure output                                    │
│  4. THEN write implementation                                   │
│  5. Run the test → SEE IT PASS                                  │
│                                                                 │
│  If I skip steps 1-3, I am VIOLATING the protocol.              │
└─────────────────────────────────────────────────────────────────┘
```

---

## CURRENT STATUS

```
make test       - PASS
make lint       - 0 issues ✅
make functional - 37 encoding + 14 api + 10 parsing + 18 decoding ✅
```

---

## Resume Point

**Last worked:** 2026-01-03
**Last commit:** `4a01500` feat(api): add msg-id to all message types (OPEN, NOTIFICATION, KEEPALIVE)

**Status:** API output format complete. Design transition document created.

---

## RECENTLY COMPLETED: Design Transition Documentation

**Created:** `plan/DESIGN_TRANSITION.md`

### Summary

Documented the Pool + Wire lazy parsing design and updated all live specs to reference it.

### Key Design Points

1. **Wire-canonical storage:** Routes store `pool.Handle` → wire bytes, not parsed `[]Attribute`
2. **Lazy parsing:** `AttributesWire.Get(AttrCode)` parses on demand
3. **Zero-copy forwarding:** `pool.Get(handle)` when `sourceCtxID == destCtxID`
4. **Memory deduplication:** Pool interns identical attribute bytes

### Spec Updates

| Spec | Update |
|------|--------|
| `DESIGN_TRANSITION.md` | **NEW** - master architecture document |
| `spec-static-route-updatebuilder.md` | Marked RIB section obsolete |
| `spec-context-full-integration.md` | Added Pool+Wire notes |
| `spec-pool-integration.md` | Marked as superseded |
| `spec-unified-handle-nlri.md` | Updated execution order |
| `spec-pool-handle-migration.md` | Marked as primary implementation spec |
| `spec-adjribout-memory-profiling.md` | Added target memory model |
| `plugin-system-mvp.md` | Added compatibility notes |

### What This Changes

| Old Plan | New Plan |
|----------|----------|
| Convert `buildRIBRouteUpdate` to UpdateBuilder | Delete it, use pool forwarding |
| Implement `spec-pool-integration.md` factories | Skip, go directly to pool handles |
| Store parsed `[]Attribute` in Route | Store `pool.Handle` instead |

### Implementation Order

```
1. spec-pool-handle-migration.md   ← PRIMARY (pool core + attr handles)
2. spec-unified-handle-nlri.md     ← NLRI as handles
3. Delete buildRIBRouteUpdate      ← Replaced by pool forwarding
```

---

## RECENTLY COMPLETED: Message Direction

**Spec:** `plan/spec-message-direction.md`

### Summary
Added `direction` ("sent"/"received") to all BGP message API output (OPEN, NOTIFICATION, KEEPALIVE, UPDATE).

### Changes
1. **RawMessage.Direction** - `pkg/api/types.go`
   - New `Direction string` field ("sent" or "received")
   - Updated comment to reflect sent/received usage

2. **MessageCallback signature** - `pkg/reactor/session.go`
   - Added `direction string` parameter
   - `processMessage()` passes "received"
   - `writeMessage()` passes "sent" after successful send

3. **Text formatters** - `pkg/api/text.go`
   - `FormatOpen(peer, open, direction)`
   - `FormatNotification(peer, notify, direction)`
   - `FormatKeepalive(peer, direction)`
   - `formatFilterResultText(peer, result, updateID, direction)`
   - `formatRawFromResult` includes direction

4. **JSON formatters** - `pkg/api/json.go`
   - `Open()`, `Notification()`, `Keepalive()` accept direction
   - `formatFilterResultJSON()` includes `"direction":"..."` field

5. **Reactor integration** - `pkg/reactor/reactor.go`
   - `notifyMessageReceiver()` accepts direction, sets `msg.Direction`

### Output Format
```
peer 10.0.0.1 sent open asn 65000 router-id 2.2.2.2 hold-time 90
peer 10.0.0.1 received open asn 65001 router-id 1.1.1.1 hold-time 90
peer 10.0.0.1 received update 1 announce origin igp ...
peer 10.0.0.1 sent keepalive
peer 10.0.0.1 received notification code 6 subcode 2 ...
```

### Verification
```
make test       - PASS
make lint       - 0 issues ✅
make functional - 37 + 14 + 10 + 18 passed ✅
```

---

## RECENTLY COMPLETED: AFI/SAFI Map-Based Refactor

**Spec:** `plan/spec-afi-safi-map-refactor.md`

### Summary
Consolidated Family type to nlri package, separated "what was negotiated" from "how to encode",
eliminated data duplication between NegotiatedFamilies and EncodingContext.

### Changes
1. **Family type consolidation** - capability.AFI/SAFI/Family now alias nlri types
2. **NegotiatedCapabilities** - replaces NegotiatedFamilies (25 bools → 1 map)
3. **ExtendedNextHop type change** - now `map[Family]AFI` (stores next-hop AFI, not bool)
4. **EOR deduplication** - removed duplicate EOR sending from family-specific functions
5. **Deterministic ordering** - `Families()` returns sorted slice for reproducible EOR order

### Code Reduction
- ~180 lines removed from peer.go
- Eliminated 6 switch statements checking AFI+SAFI pairs

### Verification
```
make test       - PASS
make lint       - 0 issues ✅
make functional - 79/79 passed (100%) ✅
```

---

## RECENTLY COMPLETED: Peer Encoding Extraction

**Spec:** `plan/spec-peer-encoding-extraction.md`

### Summary
Fixed production bugs and cleaned up ~500 LOC of dead code:

1. **ORIGINATOR_ID/CLUSTER_LIST bug** - RFC 4456 attributes silently dropped in non-grouped paths
2. **routeGroupKey bug** - Missing reflector fields caused silent data loss during UPDATE grouping
3. **ASN4 encoding bug (RFC 6793)** - AS_PATH always used 4-byte encoding even when `asn4 disable` was set
   - Fixed in ALL builders: Unicast, GroupedUnicast, VPN, LabeledUnicast, MVPN, VPLS, FlowSpec, MUP

### Changes
1. **UnicastParams extended** - `pkg/bgp/message/update_build.go`
   - Added `OriginatorID uint32` and `ClusterList []uint32` fields

2. **BuildUnicast fixed** - Encodes ORIGINATOR_ID (type 9) and CLUSTER_LIST (type 10)

3. **BuildGroupedUnicast added** - New method for grouped IPv4 unicast with shared attributes

4. **routeGroupKey fixed** - Now includes OriginatorID and ClusterList in grouping key

5. **ASN4 encoding fixed in ALL builders** - AS_PATH now uses correct 2-byte or 4-byte encoding:
   - BuildUnicast, BuildGroupedUnicast (already fixed)
   - BuildVPN, BuildLabeledUnicast, BuildMVPN, BuildVPLS, BuildFlowSpec, BuildMUP, BuildMUPWithdraw

6. **AGGREGATOR ASN4 encoding fixed** - AGGREGATOR uses correct 6-byte or 8-byte format:
   - Uses 2-byte ASN when ASN4=false, 4-byte ASN when ASN4=true
   - Uses AS_TRANS (23456) for large ASNs when ASN4=false (RFC 6793 Section 4.2.3)
   - Fixed in: BuildUnicast, BuildGroupedUnicast, BuildVPN, BuildLabeledUnicast

7. **sendInitialRoutes updated** - Uses new BuildGroupedUnicast for multi-route groups

8. **Wire compat tests migrated** - Changed from old-vs-new comparison to expected-bytes assertions

9. **Dead code deleted (~500 LOC)**:
   - `buildStaticRouteUpdate` (old UPDATE builder)
   - `buildGroupedUpdate` (old grouped builder)
   - `buildMPReachNLRI` (VPN MP_REACH)
   - `buildVPNNLRIBytes` (VPN NLRI helper)
   - `buildMPReachNLRIExtNHUnicast` (extended NH helper)
   - `buildMPReachNLRIUnicast` (IPv6 MP_REACH)
   - `TestBuildMPReachNLRIUnicast` (obsolete test)

10. **12 new ASN4 unit tests** - Verifying 2-byte AS/AGGREGATOR encoding for all route types

### Verification
```
make test       - PASS
make lint       - 0 issues ✅
make functional - 37/37 passed (100%) ✅
```

---

## RECENTLY COMPLETED: Environment Configuration Block

**Spec:** `plan/spec-environment-config-block.md`

### Summary
ZeBGP-specific feature to set environment variables from config file instead of shell.
Includes strict validation (BREAKING CHANGE from ExaBGP's silent-ignore behavior).

### Changes
1. **Strict parsing functions** - `pkg/config/environment.go`
   - `parseBoolStrict()`, `parseIntStrict()`, `parseFloatStrict()`, `parseOctalStrict()`
   - Return errors instead of silent defaults

2. **Validation functions** - `pkg/config/environment.go`
   - `validateLogLevel()` - DEBUG, INFO, NOTICE, WARNING, ERR, CRITICAL
   - `validatePort()` - 1-65535
   - `validateEncoder()` - json, text
   - `validateAttempts()` - 0-1000
   - `validateOpenWait()` - 1-3600
   - `validateSpeed()` - 0.1-10.0

3. **Table-driven setters** - `envOptions` map
   - All 8 sections: daemon, log, tcp, bgp, cache, api, reactor, debug
   - Backward compat: `tcp.once`, `tcp.connections` aliases

4. **New API** - `pkg/config/environment.go`
   - `SetConfigValue(section, option, value)` - set individual values
   - `LoadEnvironmentWithConfig(map)` - load with config block values
   - `LoadEnvironment() (*Environment, error)` - now returns error (BREAKING)

5. **Config parser integration** - `pkg/config/bgp.go`
   - `environmentBlock()` schema for BGPSchema and LegacyBGPSchema
   - `ExtractEnvironment(tree)` extracts values from parsed config
   - `BGPConfig.EnvValues` field carries environment values

6. **Migration helper** - `cmd/zebgp/config_check.go`
   - `--env` flag validates environment variables before upgrade
   - JSON output support

7. **50+ new tests** - `pkg/config/environment_test.go`
   - Strict parsing, validation, SetConfigValue, LoadEnvironmentWithConfig
   - Backward compat (tcp.once, tcp.connections)
   - Environment block parsing

8. **Documentation** - `.claude/zebgp/config/`
   - `ENVIRONMENT_BLOCK.md` - new feature doc
   - Updated `SYNTAX.md` - added environment section
   - Updated `ENVIRONMENT.md` - added ZeBGP enhancements

### Usage
```
environment {
    log { level DEBUG; }
    tcp { port 1179; }
    api { encoder text; }
}
```

Priority: OS env > config block > defaults

### Verification
```
make test       - PASS
make lint       - 0 issues ✅
```

---

## RECENTLY COMPLETED: Listener Per Local Address

**Spec:** `plan/done/spec-listener-per-local-address.md`

### Summary
Replace single listener + `TCP.Bind` with multiple listeners derived from peer
`LocalAddress` fields. Security improvement - only expose BGP on configured interfaces.

### Changes
1. **Multi-listener map** - `pkg/reactor/reactor.go`
   - `listeners map[netip.Addr]*Listener` (one per unique LocalAddress)
   - `startListenerForAddress()`, `stopAllListeners()` helpers
   - `handleConnectionWithContext()` validates LocalAddress match

2. **Dynamic lifecycle** - AddPeer/RemovePeer
   - AddPeer creates listener if running and new LocalAddress
   - RemovePeer stops listener when last peer using it removed

3. **LocalAddress validation**
   - Self-referential (Address == LocalAddress) → rejected
   - Link-local IPv6 → rejected
   - Address family mismatch (IPv4/IPv6) → rejected

4. **IPv4-mapped IPv6 normalization**
   - Both Address and LocalAddress unmapped in AddPeer
   - RemovePeer also normalizes for consistent lookup
   - Connection handler unmaps incoming IPs

5. **TCP.Bind removed** - `pkg/config/environment.go`
   - `Bind []string` field removed from TCPEnv
   - `getEnvStringList` helper removed (unused)

6. **19 new tests** - `pkg/reactor/reactor_test.go`
   - Multi-listener startup (5 tests)
   - LocalAddress validation (4 tests)
   - Dynamic lifecycle (4 tests)
   - IPv4-mapped handling (6 tests)

### Verification
```
make test       - PASS
make lint       - pre-existing dupl only (6)
```

---

## RECENTLY COMPLETED: Labeled-Unicast API Completeness (`4c16628`)

**Spec:** `plan/done/spec-labeled-unicast-api-completeness.md`

### Summary
Complete labeled-unicast (SAFI 4) API to match AnnounceRoute pattern with full
Adj-RIB-Out tracking, transaction support, and all path attributes preserved.

### Changes
1. **nlri.LabeledUnicast type** - `pkg/bgp/nlri/labeled.go`
   - Implements `nlri.NLRI` interface
   - RFC 8277 wire format, RFC 3032 label encoding
   - RFC 7911 ADD-PATH support, label stack support

2. **PathID in API** - `pkg/api/types.go`, `route.go`, `route_keywords.go`
   - Added `PathID uint32` to `LabeledUnicastRoute`
   - Added `path-id` keyword parsing

3. **3-way switch pattern** - `pkg/reactor/reactor.go`
   - `AnnounceLabeledUnicast`: Transaction → Established → Queue
   - `WithdrawLabeledUnicast`: Same pattern
   - `buildLabeledUnicastRIBRoute`: ALL attributes (not just Origin)

4. **Bug fixes**
   - Fixed `buildLabeledUnicastNLRIBytes` ADD-PATH pathID=0 (RFC 7911)
   - Added LocalPref to `buildLabeledUnicastRIBRoute`

5. **Wire consistency tests** - `pkg/bgp/message/labeled_wire_test.go`
   - Verifies `nlri.LabeledUnicast.Pack()` matches `buildLabeledUnicastNLRIBytes`

6. **Unit tests for attribute storage** - `pkg/reactor/reactor_test.go`
   - `TestBuildLabeledUnicastRIBRouteAllAttributes`
   - `TestBuildLabeledUnicastRIBRouteIBGPDefaults`
   - `TestBuildLabeledUnicastRIBRouteEBGPPrependsAS`

### Verification
```
make test       - PASS
make lint       - pre-existing dupl only (6)
```

---

## RECENTLY COMPLETED: MUP API Fixes (`794eb8e`)

**Spec:** `plan/done/spec-mup-api-fixes.md` ✅

### Changes
1. **Family validation** in `buildAPIMUPNLRI()` - all 4 route types validated
2. **Unit tests** - `pkg/reactor/mup_test.go` with 24 test cases

### Verification
```
make test       - PASS
make lint       - pre-existing dupl only (6)
API functional  - 14/14 passed
```

---

## RECENTLY COMPLETED: MUP API Support (`64d8a0f`)

Full MUP SAFI (85) support for API commands:
- `announce ipv4/ipv6 mup mup-isd <prefix> rd <RD> next-hop <NH> extended-community [...] bgp-prefix-sid-srv6 (...)`
- `withdraw ipv4/ipv6 mup mup-isd <prefix> rd <RD> next-hop <NH> extended-community [...] bgp-prefix-sid-srv6 (...)`

Route types: mup-isd, mup-dsd, mup-t1st, mup-t2st

### Next steps
1. Move spec to done/: `git mv plan/spec-mup-api-support.md plan/done/`
2. Update plan/README.md with completed MUP API entry

---

## RECENT FIX: check test (version 6 format)

### Misdiagnosis Corrected
Previous diagnosis: "Static routes not being sent" - **WRONG**

Actual finding: Static routes and EORs WERE being sent correctly. The issue was
that check.run wasn't receiving forwarded UPDATEs in the expected format.

### Root Cause
The check.run script expects ExaBGP-compatible format (version 6):
```
neighbor 127.0.0.1 receive update announced 0.0.0.0/32 next-hop 127.0.0.1 origin igp local-preference 100
```

But ZeBGP defaults to version 7 format:
```
peer 127.0.0.1 update announce nlri ipv4 unicast 0.0.0.0/32 next-hop 127.0.0.1 ...
```

### Fix Applied
Added `version 6;` to check.conf content block (test data fix, not code fix):
```
api check-and-announce {
    content {
        format parsed;
        version 6;  // <-- Added
    }
    ...
}
```

### Remaining Skipped Tests
- **announcement** - Multi-session qualifiers not supported by design (SKIP)

---

## TECHNICAL DEBT

### ✅ 1. Unit tests for mergeAPIBindings() - DONE
Location: `pkg/config/bgp.go:1416`
- Added 8 unit tests in `bgp_test.go` (TestMergeAPIBindings*)
- Tests cover: empty inputs, append, replace, mixed, order preservation

### ✅ 2. Unit tests for template inheritance - DONE
- Added 6 unit tests for API binding inheritance
- Tests cover: inherit, peer override, multiple processes, match templates
- **BUG FIXED:** Match templates were not applying API bindings
- **OPTIMIZED:** Collect matching trees once, reuse for API bindings (avoids duplicate iteration)

### 3. Functional test reporter message merging bug (Priority: Low)
Location: `test/functional/record.go`
- All messages in check.ci use index `1:`, causing them to merge
- Report shows wrong "EXPECTED MESSAGE 1" (shows last message only)
- Actual testpeer comparison is correct (order-agnostic)
- Only affects diagnostic output, not test correctness

### 4. check.ci order documentation mismatch (Priority: Low)
- CI file shows: EOR → EOR → routes
- ZeBGP sends: routes → EOR → EOR
- Both are valid BGP (testpeer is order-agnostic)
- CI comments are misleading

### 5. Multiple inherit not supported (Priority: Low - design limitation)
- `inherit` is defined as `Leaf(TypeString)`, not a List
- Second `inherit` statement overwrites first
- Workaround: use single template with multiple api blocks

### Spec Location
`plan/spec-api-test-features.md` - Updated with current status

---

## PLANNED

### Pool + Wire Design (Priority)

See `plan/DESIGN_TRANSITION.md` for overall architecture.

| Spec | Description | Status |
|------|-------------|--------|
| `spec-pool-handle-migration.md` | Pool core + Route stores Handle | **PRIMARY** |
| `spec-unified-handle-nlri.md` | NLRI as 4-byte Handle | After pool-handle |
| `spec-context-full-integration.md` | Zero-copy forwarding | Partial (needs pool) |

### Superseded (Skip)

| Spec | Reason |
|------|--------|
| `spec-pool-integration.md` | Factory methods obsolete, go directly to pool handles |
| `spec-static-route-updatebuilder.md` (RIB section) | Use pool forwarding, not UpdateBuilder |

### Other Specs

| Spec | Description | Status |
|------|-------------|--------|
| `spec-rfc9234-role.md` | RFC 9234 Role for API policy | Ready (independent) |
| `spec-adjribout-memory-profiling.md` | Memory profiling | Run after pool impl |
| `plugin-system-mvp.md` | Plugin system | Ready (compatible) |
| `phase0-peer-callbacks.md` | Peer lifecycle | Ready (independent) |

### Recently Completed (in plan/done/)
- `spec-attributes-wire.md` - Wire-canonical storage ✅
- `spec-route-id-forwarding.md` - Route ID forwarding ✅
- `spec-adj-rib-out-forward.md` - Adj-RIB-Out forward ✅
- `debug-teardown-timing.md` - Teardown timing issue ✅

---

## COMPLETED

### Decoding Tests Complete (`196c95d`)
**Spec:** `plan/done/spec-functional-decoding-parsing.md` ✅ COMPLETE

All 18 decoding tests now pass with lossless JSON format:

**TLV 1099 (SR-MPLS Adjacency SID):**
- Implemented RFC 9085 Section 2.2.1 parsing
- V/L flag handling for 3-byte label vs 4-byte index
- Array format for multiple TLV instances (lossless)
- Unit tests: `TestParseSRMPLSAdjSID`, `TestSRAdjMultipleInstances`

**Lossless JSON Format (API breaking change):**
- `remote-router-id` → `remote-router-ids` (array, preserves IPv4+IPv6)
- `sr-adj` → array format (preserves multiple TLV instances)
- `srv6-endx-sid` → `srv6-endx` (key name fix)

**Test file updates:**
- `bgp-ls-5.test`: sr-adj array format
- `bgp-ls-6..9.test`: lossless router-ids format

**RFC downloaded:**
- `rfc/rfc9085.txt` (BGP-LS Extensions for Segment Routing)
- `rfc/README.md` updated with BGP-LS section

**ExaBGP sync required:** Same duplicate-key bug exists in ExaBGP. See spec for fix plan.

### BGP-LS Decode + SRv6 TLVs (`48fe442`, `7054c27`, `4ff9140`)
**Spec:** `plan/spec-functional-decoding-parsing.md`

Complete BGP-LS structured JSON output for decode command:
- RFC 7752: Node, Link, Prefix NLRI types
- Node descriptors (AS, BGP-LS ID, OSPF Area, Router-ID)
- Link descriptors (interface/neighbor addresses, MTIDs, link IDs)
- BGP-LS attribute (type 29) TLV parsing
- RFC 9514 SRv6 TLVs: 1106, 1107, 1108, sub-TLV 1252
- Fixed formatNodeDescriptors to only include present TLVs
- Unified IPv6 formatting with compression
- Added originator-id and cluster-list attribute parsing

Files:
- `cmd/zebgp/decode.go`, `decode_test.go`
- `rfc/rfc7752.txt`, `rfc/rfc9514.txt`

### MUP T1ST Source Field + Fail-Early Rule (`f6846fb`)

Added source field support for MUP T1ST routes (test U):
- `MUPRouteConfig.Source` field for T1ST source address
- Parsing in `parseMUPFromInline` and `parseMUPRoute`
- Encoding in `buildMUPNLRI`: source_len (1B) + source_addr (variable)

Refactored `buildMUPNLRI` for fail-early error handling:
- Now returns `([]byte, error)` instead of `[]byte`
- All parse failures report specific errors (prefix, address, endpoint, source, RD)
- Missing required fields return descriptive errors

Added coding standard rule:
- `.claude/CODING_STANDARDS.md`: "Fail Early and Loud" blocking rule
- Configuration/parsing errors MUST propagate, never silently ignored

Added unit test:
- `TestBuildMUPNLRI_T1ST_Source`: 6 test cases for source encoding and error handling

Fixed pre-existing lint issues:
- `routeattr.go:421`: Added period to comment
- `routeattr.go:1023`: Converted if-else chain to switch
- `routeattr.go:1067`: Extracted isHexDigit variable

Files:
- `.claude/CODING_STANDARDS.md`
- `pkg/config/bgp.go`
- `pkg/config/loader.go`
- `pkg/config/loader_test.go`
- `pkg/config/routeattr.go`

### Functional Test System Migration (uncommitted)
**Spec:** `plan/spec-functional-test-diagnostics.md` ✅ COMPLETE

Migrated functional test system from old to new implementation:
- Removed old `test/cmd/functional/`, `test/cmd/self-check/`, `test/pkg/`
- Renamed `test/cmd/selfcheck/` → `test/cmd/functional/`
- Renamed `test/selfcheck/` → `test/functional/`
- Package renamed from `selfcheck` to `functional`
- Added `--count N` flag for stress testing
- Added `--save DIR` flag for log capture
- AI-friendly failure reports with decoded messages
- Dynamic port allocation, ulimit auto-raise

Files:
- New package: `test/functional/` (12 files)
- Entry point: `test/cmd/functional/main.go`
- Updated: `Makefile`

### Format-Based Migration Refactor (uncommitted)
**Spec:** `plan/spec-format-based-migration.md` ✅ COMPLETE

Refactored migration from version-based to transformation-based:
- `MigrateV2ToV3()` → `Migrate()` returning `*MigrateResult`
- `DetectVersion()` → `NeedsMigration()` returning `bool`
- Removed `ConfigVersion`, `Version2`, `Version3` constants
- New CLI flags: `--dry-run`, `--list`
- Transformation visibility: Applied/Skipped lists with emoji status
- Atomic: all transformations succeed or original unchanged

Files changed:
- New: `pkg/config/migration/migrate.go`, `migrate_test.go`
- Removed: `version.go`, `v2_to_v3.go`
- Updated: `detect.go`, `cmd/zebgp/config_migrate.go`, `config_check.go`, `config_fmt.go`

### API Encoder Switching (`d330456`)
**Spec:** `plan/spec-api-encoder-switching.md` ✅ COMPLETE

Per-peer API binding with encoding/format control:
- **Phase 0:** Message dispatch for all BGP message types
- **Phase 1:** Config parsing with new syntax `api <name> { content {...} receive {...} }`
- **Phase 2:** Per-peer message routing with bindings
- **Phase 3:** Output format (v6/v7 JSON, text)
- **Phase 4:** Migration tool (`zebgp config migrate` handles api blocks)
- **Phase 5:** Documentation updates

Key features:
- Named api blocks: `api foo { content { encoding json; } receive { update; } }`
- Encoding inheritance: peer → process → "text" default
- `all;` keyword expansion in receive/send blocks
- Error on empty/duplicate/collision in migration
- 16 migration tests, 8 config tests

### Zero-Copy Forwarding (`a317ea9`)
**Spec:** `plan/spec-context-full-integration.md` Phase 3
**Docs:** `.claude/zebgp/ENCODING_CONTEXT.md`

Added ID-only API for route forwarding:
- `PackAttributesFor(destCtxID)` - zero-copy or re-encode attributes
- `PackNLRIFor(destCtxID)` - zero-copy or re-encode NLRI
- `NewRouteWithWireCacheFull()` - cache both attributes and NLRI
- `nlriWireBytes` field in Route for NLRI zero-copy
- Registry lookup for slow path
- Pre-allocated buffer in packAttributesWithContext
- 11 TDD tests (including edge cases)

### Route Wire Cache (`d4c0fb2`)
**Spec:** `plan/spec-context-full-integration.md` Phase 2

Added wire cache to Route for zero-copy forwarding:
- `wireBytes` and `sourceCtxID` fields
- `NewRouteWithWireCache()` constructor
- `CanForwardDirect()` eligibility check
- 5 TDD tests

### PackWithContext on Attribute Interface (`d8836c9`)
**Spec:** `plan/spec-context-full-integration.md` Phase 4

Added `PackWithContext(srcCtx, dstCtx *EncodingContext)` to Attribute interface:
- ASPath: context-dependent (uses dstCtx.ASN4)
- Aggregator: context-dependent (8-byte vs 6-byte format)
- All others: delegate to Pack()
- 14 TDD tests

### Peer EncodingContext Integration (`ae85931`)
**Spec:** `plan/spec-context-full-integration.md` Phase 1

- `Peer.recvCtx/sendCtx` fields
- `FromNegotiatedRecv/Send()` helpers
- Context lifecycle (set on establish, clear on teardown)
- 35 tests covering ADD-PATH, ASN4, encoding

### EncodingContext Package (`1afd604`)
**Spec:** `plan/spec-encoding-context-impl.md` ✅ COMPLETE

`pkg/bgp/context/` package:
- `EncodingContext` struct with ASN4, AddPath, ExtendedNextHop
- `ContextID` for fast comparison
- `ContextRegistry` with hash deduplication
- `FromNegotiatedRecv/Send()` helpers
- 13 tests

### Earlier Commits

| Commit | Feature |
|--------|---------|
| `3a8ef7b` | Keyword validation for FlowSpec, VPLS, L2VPN |
| `f34bac0` | ADD-PATH support for VPN routes (test 0 fixed) |
| `9c94a2b` | Static route building to use UpdateBuilder |
| `53b8d12` | Extract UPDATE builders to message package |
| `13fd04b` | Add ASN4 to PackContext (RFC 6793) |
| `81b9ed9` | Rename NLRIHashable.Bytes() to Key() |
