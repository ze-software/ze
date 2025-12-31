# Claude Continuation State

**Last Updated:** 2025-12-31

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
make test   - PASS
make lint   - PASS
functional  - 37/37 passed
decoding    - 18/18 passed
parsing     - 10/10 passed
```

---

## Resume Point

**Last worked:** 2025-12-31
**Last commit:** uncommitted (decoding tests complete)
**Session ended:** Clean break - all decoding tests pass

**Next steps:**
1. Commit decoding test completion
2. Sync ExaBGP with lossless JSON format (see spec)
3. API as Virtual Peer implementation

---

## PLANNED

### API as Virtual Peer
**Spec:** `plan/spec-api-virtual-peer.md`

Add EncodingContext to API connections:
- API clients declare capabilities (ASN4, ADD-PATH)
- Context locked after first route
- Routes from API get sourceCtxID for zero-copy

### Wire Container (Future)
**Spec:** `plan/spec-attribute-context-wire-container.md`

AttributesWire for zero-copy route reflection.

---

## COMPLETED

### Decoding Tests Complete (uncommitted)
**Spec:** `plan/spec-functional-decoding-parsing.md` ✅ COMPLETE

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
