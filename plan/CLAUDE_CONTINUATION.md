# Claude Continuation State

**Last Updated:** 2025-12-30

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
functional  - 33 passed, 4 failed [S, V, U, a]
```

---

## Resume Point

**Last worked:** 2025-12-30
**Last commit:** (uncommitted) functional test migration
**Session ended:** Functional test system migration complete

**Next steps:**
1. Fix remaining functional tests [S, V, U, a]
2. Integrate zero-copy forwarding into peer route distribution (future)

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
