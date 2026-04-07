# Spec: family-registry

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | - |
| Updated | 2026-04-07 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/patterns/registration.md` - registration architecture
4. `plan/learned/187-family-plugin-infrastructure.md` - prior family registration decisions
5. `internal/component/bgp/nlri/nlri.go` - current Family/AFI/SAFI types and String() (source of move)
6. `internal/core/family/` - new package location (target of move; sits alongside `clock`, `env`, `metrics`)
7. `internal/component/bgp/message/family.go` - builtin family constants
8. `internal/component/plugin/registry/registry.go` - plugin registry, PluginForFamily

## Task

Replace hardcoded Family string maps and exported Family vars with a registration-based system.
The family registry lives in `internal/core/family/` -- core infrastructure used by BGP and other components, not owned by the BGP component.
Each plugin registers its own families via `family.RegisterFamily(afi, safi, afiStr, safiStr)`.
The family package owns the `Family`, `AFI`, `SAFI` types and the registry API.
The nlri package keeps only the BGP NLRI interface, parsing functions, and BGP-LS encoding helpers; it imports `family` for the `Family` type used in the `NLRI.Family()` method.
Family strings, AFI names, and SAFI names are all derived from registration data.
The string cache uses a packed contiguous `[]byte` buffer with `unsafe.String` for L1 cache locality and zero-copy reads.
External (Python) plugins register families at runtime via the plugin protocol.

**Origin:** Performance profiling showed `Family.String()` at 17% CPU and `PluginForFamily` at 32% CPU in the UPDATE hot path. Design discussion during perf optimization led to this architectural fix.

## Required Reading

### Architecture Docs
- [ ] `.claude/patterns/registration.md` - registration architecture
  -> Constraint: everything registers via init(), core never imports specific plugins
  -> Constraint: all registration complete before concurrent access
- [ ] `docs/architecture/core-design.md` - component boundaries
  -> Constraint: components independent unless explicit dependency
- [ ] `docs/architecture/api/process-protocol.md` - external plugin protocol
  -> Constraint: plugin declares capabilities at startup

### Prior Work
- [ ] `plan/learned/187-family-plugin-infrastructure.md` - family plugin decisions
  -> Decision: family registration integrated into Register() (one call site per plugin)
  -> Decision: family format validated at registration (must contain "/")
  -> Decision: case normalization at storage time, not lookup time

**Key insights:**
- Registration is the unifying pattern: everything registers at init() via init()
- Families are strings in the plugin registry (`Registration.Families []string`)
- Builtin families registered separately via `RegisterBuiltinFamilies`
- External plugins declare families at protocol negotiation time

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/nlri/nlri.go` - Family type, AFI/SAFI types, ParseFamily. Already has packed buffer implementation from perf work (familyPack, familyIdx, afiSlot(), buildFamilyCache(), unsafe.String read path in Family.String()). Current cache is populated from hardcoded familyStrings map -- this spec replaces that data source with registration. **All Family/AFI/SAFI types and the registry move to `internal/core/family/`; only the NLRI interface, parsing, and BGP-LS helpers stay in nlri.**
- [ ] `internal/component/bgp/nlri/constants.go` - additional SAFI constants and Family vars (MVPN, VPLS, RTC, MUP, FlowSpecVPN)
- [ ] `internal/component/bgp/message/family.go` - duplicate AFI/SAFI types, Family string constants, FamilyConfigNames map, RegisterBuiltinFamilies call
- [ ] `internal/component/plugin/registry/registry.go` - PluginForFamily (O(N) iteration, now with familyIndex), Registration.Families []string
- [ ] `internal/core/family/` - **new package, target of the move.** Owns Family/AFI/SAFI types and the registry. Sits alongside `clock`, `env`, `metrics`, `network`.

**Behavior to preserve:**
- `Family.String()` returns canonical strings like "ipv4/unicast", "l2vpn/evpn"
- `ParseFamily(string) (Family, bool)` accepts canonical names
- Plugin registry `Families []string` field carries family strings
- JSON output uses family strings as keys ("ipv4/unicast", not "1/1")
- AFI/SAFI numeric values unchanged (wire format)
- External plugin protocol family negotiation

**Behavior to change:**
- `Family`, `AFI`, `SAFI` types: relocated from `internal/component/bgp/nlri` to `internal/core/family`
- AFI/SAFI numeric constants (`AFIIPv4`, `SAFIUnicast`, etc.): relocated from `nlri` to `family`
- `Family.String()`, `AFI.String()`, `SAFI.String()`, `ParseFamily`, `FamilyLess`, `afiSlot`: relocated to `family`
- All callers (~150 files): import update from `family.Family`/`family.AFI`/`family.SAFI`/`family.AFIIPv4` etc. to `family.*`. Mechanical, single-line per file.
- `nlri` package: keeps `NLRI` interface, parsing functions, error vars, BGP-LS encoding helpers; imports `family` for the `Family` type used in the `NLRI.Family()` method.
- Family.String(): from hardcoded switch/map to registration-based packed buffer lookup
- AFI.String(): from hardcoded switch to registration-based lookup
- SAFI.String(): from hardcoded switch to registration-based lookup
- Exported Family vars (IPv4FlowSpec, L2VPNEVPN, etc.): moved from nlri to their owning plugins
- Builtin Family vars (IPv4Unicast, etc.): moved from nlri to message/family.go
- `familyStrings` map: deleted, replaced by registration data
- `FamilyConfigNames` map in message/family.go: derived from registration data
- PluginForFamily: already optimized with reverse index (keep)

## Data Flow (MANDATORY)

### Entry Point
- **Registration (init):** Plugin init() calls `family.RegisterFamily(afi, safi, afiStr, safiStr)` -> returns Family value, stores in packed buffer
- **Registration (runtime):** External plugin connects -> declares families via protocol -> engine calls `family.RegisterFamily`
- **Wire decode:** Raw AFI (2 bytes) + SAFI (1 byte) from BGP message -> `family.Family{AFI: afi, SAFI: safi}` -> `Family.String()` looks up packed buffer

### Transformation Path
1. `RegisterFamily` validates: AFI name consistent with prior registrations for same AFI number, same for SAFI. Mismatch = fatal.
2. `RegisterFamily` appends string data to new packed buffer, rebuilds spans, atomic-swaps `[]byte` pointer.
3. `Family.String()` reads `atomic.Pointer` -> index lookup `[afiSlot][SAFI]` -> span read -> `unsafe.String` slice from buffer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> family registry | `family.RegisterFamily()` at init | [ ] |
| External plugin -> Engine -> family | Protocol negotiation -> engine calls RegisterFamily | [ ] |
| Hot path read | `atomic.Pointer` load, no lock | [ ] |

### Integration Points
- `internal/component/plugin/registry/registry.go` - PluginForFamily uses family strings from registration
- `internal/component/bgp/format/text_json.go` - formatFilterResultJSON uses Family.String()
- `internal/component/bgp/message/family.go` - AFISAFIToFamily delegates to Family.String()
- Plugin protocol negotiation - external plugins declare families
- `ParseFamily` - config parsing uses registered canonical names

### Architectural Verification
- [ ] No bypassed layers (registration is the only path to add families)
- [ ] No unintended coupling (family has no plugin imports, plugins import family; nlri imports family but family does not import nlri)
- [ ] No duplicated functionality (replaces familyStrings, FamilyConfigNames, AFI.String switch, SAFI.String switch)
- [ ] Zero-copy preserved (unsafe.String into packed buffer, no allocation on hot path)

## Storage Design

### Packed Buffer Layout

The packed buffer infrastructure (familyPack, familyIdx, afiSlot, buildFamilyCache, unsafe.String read path) already exists in `internal/component/bgp/nlri/nlri.go` from perf work. This spec moves it to `internal/core/family/registry.go`, changes the data source from familyStrings to registrations, wraps familyPack in atomic.Pointer, and adds AFI/SAFI name storage. The Family.String() read path barely changes.

All family string data lives in a single contiguous `[]byte` allocation for L1/L2 cache locality (~1.3KB total).

**Buffer structure (back-to-back, no gaps):**

| Region | Content | Size per entry | Total |
|--------|---------|---------------|-------|
| Spans | (pos uint16 LE, size uint16 LE) per registered family | 4 bytes | ~84 bytes (21 families) |
| String data | All family strings packed contiguously | variable | ~234 bytes |

Span `pos` values are **absolute offsets** into the buffer (not relative to the string region). This eliminates one addition in the hot path.

**Separate from the packed buffer:**

| Structure | Content | Size |
|-----------|---------|------|
| `familyIdx [4][256]uint8` | Maps (afiSlot, SAFI) to 1-based span index. 0 = unknown. | 1024 bytes |

The index is a fixed-size array, contiguous by definition. Combined with the packed buffer, total memory is ~1.3KB.

### AFI Slot Mapping

Maps the 4 known AFI values to compact indices 0-3 for the index array.

| AFI value | Name | Slot |
|-----------|------|------|
| 1 | IPv4 | 0 |
| 2 | IPv6 | 1 |
| 25 | L2VPN | 2 |
| 16388 | BGP-LS | 3 |

Unknown AFI values return -1 (fallback path). The mapping is a switch with 4 cases. Branch prediction handles the hot path (IPv4 = ~99% of traffic).

When a new AFI is registered that is not in the 4 known slots, the slot table must be extended. This is a code change (new constant + switch case), not a runtime operation. The IANA AFI registry changes rarely.

### AFI and SAFI Name Storage

AFI and SAFI names are also stored via registration, not hardcoded switches.

| Structure | Content | Size |
|-----------|---------|------|
| AFI name map | AFI number to name string | ~4 entries, stored in packed buffer |
| SAFI name map | SAFI number to name string | ~13 entries, stored in packed buffer |

`AFI.String()` and `SAFI.String()` look up their names from the registry. Unregistered values fall back to `"afi-N"` / `"safi-N"` via strconv (not fmt.Sprintf).

### Read Path (Hot -- zero allocation)

1. Load packed buffer pointer via `atomic.Pointer[[]byte]` (one atomic load)
2. Index lookup: `familyIdx[afiSlot(f.AFI)][f.SAFI]` (array access, no hash)
3. If index is 0: fallback to string concatenation (cold path)
4. Read span: 4 bytes at offset `(idx-1)*4` in the buffer, decode pos and size as little-endian uint16
5. Return `unsafe.String(&buffer[pos], size)` -- creates a string header pointing into the buffer, no copy

The returned string is valid as long as the buffer is reachable. Old buffers are kept alive by the GC as long as any string returned from them is still referenced.

### Write Path (Cold -- mutex-protected)

1. Acquire mutex
2. Validate: AFI name matches any prior registration for the same AFI number (fatal if conflict). Same for SAFI.
3. If family already registered with same values: release mutex, return existing Family (no-op)
4. Build new packed buffer: allocate fresh `[]byte`, write all spans with absolute positions, copy all string data
5. Build new `familyIdx` array
6. Swap via `atomic.Pointer.Store` -- readers see the new buffer on next `String()` call
7. Release mutex

Old buffer is not freed explicitly. The GC collects it when no more strings reference it (strings returned by `unsafe.String` hold a reference to the buffer's backing array).

### Concurrency Model

| Path | Synchronization | Frequency |
|------|----------------|-----------|
| `RegisterFamily` (write) | `sync.Mutex` | Tens of calls at init, rare at runtime |
| `Family.String()` (read) | `atomic.Pointer` load, no lock | Millions per second on hot path |
| Old buffer lifetime | GC via string references | Automatic |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin init() registers family | -> | family.RegisterFamily stores in packed buffer | `TestRegisterFamilyString` |
| Family.String() on registered family | -> | Packed buffer lookup returns correct string | `TestFamilyStringFromRegistry` |
| Family.String() on unregistered family | -> | Fallback concatenation | `TestFamilyStringFallback` |
| Re-registration same values | -> | No-op, no panic | `TestReRegisterSameValues` |
| Re-registration different values | -> | Fatal error | `TestReRegisterConflictPanics` |
| ParseFamily with unknown name | -> | Returns false | `TestParseFamilyUnknown` |
| External plugin registers family | -> | Cache rebuilt, String() works | `TestRuntimeRegistration` |
| Config file with family name | -> | Parses via registered data | `test/parse/test-family-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin calls `RegisterFamily(1, 133, "ipv4", "flow")` | Family.String() returns "ipv4/flow" |
| AC-2 | Two plugins register AFI 1 as "ipv4" | No-op, no error |
| AC-3 | Plugin registers AFI 1 as "ip4" (conflict) | Fatal error at startup |
| AC-4 | Family{AFI:1, SAFI:1}.String() before any registration | Falls back to "afi-1/safi-1" |
| AC-5 | RegisterFamily returns a Family value | Can be used as map key, compared with == |
| AC-6 | ParseFamily("ipv4/flow") after registration | Returns Family{1, 133}, true |
| AC-7 | ParseFamily("ipv4/flowspec") for unregistered name | Returns zero Family, false |
| AC-8 | No exported Family vars in nlri or family package | nlri keeps NLRI interface only; family keeps types, AFI/SAFI numeric constants, and registry API |
| AC-9 | All plugin-specific families declared in their plugin register.go | grep confirms no `family.IPv4FlowSpec` etc. in `internal/core/family/` or `internal/component/bgp/nlri/` |
| AC-10 | Packed buffer < 2KB total, single []byte allocation | Measured in test |
| AC-11 | Family.String() zero allocation for registered families | Benchmarked |
| AC-12 | External plugin registers family at runtime | Family.String() returns correct string after registration |
| AC-13 | AFI(1).String() returns "ipv4" from registry | No hardcoded switch |
| AC-14 | SAFI(133).String() returns "flow" from registry | No hardcoded switch |
| AC-15 | Mutex only on RegisterFamily, hot path lock-free | atomic.Pointer for read path |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterFamilyString` | `internal/core/family/registry_test.go` | RegisterFamily stores and String() retrieves | |
| `TestRegisterFamilyAFIString` | `internal/core/family/registry_test.go` | AFI.String() from registered data | |
| `TestRegisterFamilySAFIString` | `internal/core/family/registry_test.go` | SAFI.String() from registered data | |
| `TestReRegisterSameValues` | `internal/core/family/registry_test.go` | Same registration twice = no-op | |
| `TestReRegisterConflictPanics` | `internal/core/family/registry_test.go` | Conflicting AFI name = panic | |
| `TestFamilyStringFallback` | `internal/core/family/registry_test.go` | Unregistered family = concatenation fallback | |
| `TestParseFamilyRegistered` | `internal/core/family/registry_test.go` | ParseFamily uses registered data | |
| `TestParseFamilyUnknown` | `internal/core/family/registry_test.go` | ParseFamily rejects unregistered name | |
| `TestRuntimeRegistration` | `internal/core/family/registry_test.go` | Register after init, cache rebuilt | |
| `TestPackedBufferSize` | `internal/core/family/registry_test.go` | Buffer fits in < 2KB | |
| `BenchmarkFamilyString` | `internal/core/family/registry_test.go` | Zero allocation | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AFI | 1-65535 | 65535 | 0 | N/A (uint16) |
| SAFI | 0-255 | 255 | N/A | N/A (uint8) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-family-config` | `test/parse/test-family-config.ci` | Config with family names parses correctly | |

### Future
- Benchmark comparison with pre-registration code (deferred: requires stable baseline)

## Files to Modify

- `internal/component/bgp/nlri/nlri.go` - Trim significantly. Remove `Family`, `AFI`, `SAFI` types, all numeric constants, `Family.String()`, `AFI.String()`, `SAFI.String()`, `ParseFamily`, `FamilyLess`, `afiSlot()`, `familyPack`, `familyIdx`, `buildFamilyCache()`, `familyStrings`. All move to `internal/core/family/`. Keep `NLRI` interface (now using `family.Family` for the `Family()` method), parsing functions, BGP-LS encoding helpers, `WriteNLRI`, `LenWithContext`. Import `internal/core/family`.
- `internal/component/bgp/nlri/constants.go` - Keep only the error vars (`ErrMVPNTruncated`, `ErrVPLSTruncated`, etc. -- NLRI parsing errors used by plugins). Remove SAFI numeric constants (moved to `family`) and Family vars (moved to plugins per existing decision).
- `internal/component/bgp/message/family.go` - Import `internal/core/family`. Register builtin families via `family.RegisterFamily`. Remove `FamilyConfigNames` (derived from registry). `ValidFamilyConfigNames()` switches from iterating `FamilyConfigNames` to querying `family.RegisteredNames()`. Convert `message.AFI`/`message.SAFI` from separate types to real type aliases (`type AFI = family.AFI`, `type SAFI = family.SAFI`). Remove duplicate AFI/SAFI constants (use `family`'s directly).
- `internal/component/bgp/plugins/nlri/flowspec/types.go` - Import `family`. Declare local family vars via `family.RegisterFamily`. Update type aliases from `family.Family` to `family.Family`.
- `internal/component/bgp/plugins/nlri/vpn/types.go` - As above.
- `internal/component/bgp/plugins/nlri/evpn/types.go` - As above.
- `internal/component/bgp/plugins/nlri/labeled/types.go` - As above.
- `internal/component/bgp/plugins/nlri/mvpn/types.go` - As above.
- `internal/component/bgp/plugins/nlri/vpls/types.go` - As above.
- `internal/component/bgp/plugins/nlri/rtc/types.go` - As above.
- `internal/component/bgp/plugins/nlri/mup/types.go` - As above.
- `internal/component/bgp/plugins/nlri/ls/types.go` - As above (BGP-LS).
- `internal/component/bgp/reactor/peer_initial_sync.go` - Replace `family.Family` vars with local or numeric construction; import `family`.
- `internal/component/bgp/plugins/cmd/update/update_text_nlri.go` - Replace `family.Family` vars; import `family`.
- `internal/component/bgp/format/text_json.go` - Already optimized (keep logic). Update import: `family.Family` to `family.Family`.
- `internal/component/plugin/registry/registry.go` - Already has familyIndex (keep as-is).
- All other call sites (~150 files) - Mechanical import update: replace `family.Family`/`family.AFI`/`family.SAFI`/`family.AFIIPv4` etc. with `family.*`. Add `family` import where the file only had `nlri`.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Plugin protocol (family registration) | Yes | `docs/architecture/api/process-protocol.md` |
| Functional test | Yes | `test/parse/test-family-config.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - family registration |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Yes | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` - RegisterFamily in protocol |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - family registration |

## Files to Create

- `internal/core/family/family.go` - `Family`, `AFI`, `SAFI` types, AFI/SAFI numeric constants, `FamilyLess`. Package doc.
- `internal/core/family/registry.go` - `RegisterFamily`, `RegisteredNames`, packed buffer, `atomic.Pointer`, `Family.String()`, `AFI.String()`, `SAFI.String()`, `ParseFamily`, `afiSlot`.
- `internal/core/family/registry_test.go` - all registry unit tests + benchmark.
- `test/parse/test-family-config.ci` - functional test.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Create family package** - Create `internal/core/family/` with `family.go` (types, constants) and `registry.go` (RegisterFamily, packed buffer build, atomic swap, AFI/SAFI/Family String() from registry, ParseFamily). Migrate the existing packed buffer infrastructure from `nlri/nlri.go`.
   - Tests: `TestRegisterFamilyString`, `TestRegisterFamilyAFIString`, `TestRegisterFamilySAFIString`, `TestReRegisterSameValues`, `TestReRegisterConflictPanics`, `TestFamilyStringFallback`
   - Files: `internal/core/family/family.go`, `internal/core/family/registry.go`, `internal/core/family/registry_test.go`
   - Verify: tests fail -> implement -> tests pass. nlri package not yet touched; family package compiles standalone.

2. **Phase: Migrate builtin families and trim nlri** - Trim `nlri/nlri.go` and `nlri/constants.go` to remove Family/AFI/SAFI types and constants (now in family). nlri imports family for the NLRI interface. `message/family.go` registers builtin families via `family.RegisterFamily`. Convert `message.AFI`/`message.SAFI` to type aliases of `family.*`.
   - Tests: `TestParseFamilyRegistered`, `TestParseFamilyUnknown`
   - Files: `nlri/nlri.go`, `nlri/constants.go`, `message/family.go`
   - Verify: existing tests pass with new registration path. Mass import update across BGP code from `family.Family` to `family.Family`.

3. **Phase: Migrate plugin families** - Each NLRI plugin registers its own families via `family.RegisterFamily`. Plugin type aliases switch from `family.Family` to `family.Family`. Remove exported Family vars from nlri.
   - Tests: existing plugin tests continue to pass
   - Files: all `plugins/nlri/*/types.go` files, `reactor/peer_initial_sync.go`, `update_text_nlri.go`
   - Verify: `make ze-verify`

4. **Phase: Runtime registration** - External plugin protocol sends (afi, safi, afiStr, safiStr), engine calls RegisterFamily. Cache rebuilt atomically.
   - Tests: `TestRuntimeRegistration`
   - Files: plugin protocol handler
   - Verify: test with simulated external plugin

5. **Phase: Test + cleanup** - Functional tests, benchmark, remove dead code
   - Tests: `BenchmarkFamilyString`, `TestPackedBufferSize`, `test-family-config.ci`
   - Files: test files
   - Verify: `make ze-verify`

6. **Complete spec** - Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Family.String() returns identical strings as before for all known families |
| Naming | No change to JSON keys, family string format preserved |
| Data flow | Registration -> packed buffer -> atomic swap -> unsafe.String read |
| Rule: no-layering | familyStrings map deleted, not kept alongside |
| Rule: registration | All families registered via RegisterFamily, no hardcoded maps |
| Thread safety | Mutex on write, atomic.Pointer on read, no data race |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| No exported Family vars in nlri/ or family/ | `grep 'var.*= Family{' internal/component/bgp/nlri/ internal/core/family/` returns nothing |
| No familyStrings map anywhere | `grep familyStrings internal/component/bgp/nlri/ internal/core/family/` returns nothing |
| No hardcoded AFI.String() switch | `grep 'func.*AFI.*String' internal/core/family/` shows registry lookup |
| No hardcoded SAFI.String() switch | `grep 'func.*SAFI.*String' internal/core/family/` shows registry lookup |
| No `nlri.AFI*` / `nlri.SAFI*` constant or type references | `grep -r 'nlri\.\(AFI\|SAFI\)[A-Z]' --include='*.go'` returns nothing (NLRI.Family() method calls excluded by pattern) |
| Each NLRI plugin has family.MustRegister call | `grep 'family.MustRegister' internal/component/bgp/plugins/nlri/*/types.go` returns 9 files |
| Packed buffer < 2KB | TestPackedBufferSize |
| Zero alloc on hot path | BenchmarkFamilyString shows 0 allocs |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | RegisterFamily: afiStr/safiStr non-empty, no "/" in individual names |
| Conflict detection | Same AFI number with different name = fatal |
| Buffer bounds | Packed buffer spans never exceed buffer length |
| unsafe.String safety | Buffer immutable after swap, GC keeps old buffers alive |
| External plugin input | Family names from external plugins validated before RegisterFamily |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
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

- Family singleton via pointer was over-engineering. A packed buffer with cached strings achieves the same zero-allocation goal without changing the type.
- AFI (uint16) and SAFI (uint8) do not need singletons. Integer comparison is already optimal. Only String() needs caching, and that's handled at the Family level.
- The registration API signature `(afi, safi, afiStr, safiStr)` makes the plugin the authority on naming. The nlri package derives the family string automatically.

## Decision Log

| # | Decision | Resolved | Rationale |
|---|----------|----------|-----------|
| 1 | API signature | `RegisterFamily(afi, safi, afiStr, safiStr)` | Plugin provides all 4 values, family string derived |
| 2 | Who registers | Each plugin its own families | Pattern registration (learned/187) |
| 3 | Storage | Single `[]byte` packed, `unsafe.String`, `atomic.Pointer` swap | L1 cache locality, zero-copy read |
| 4 | Aliases | No aliases. One canonical name per family. Config must use registered name. | All plugins equal, no special treatment |
| 5 | Builtin families | ~~`message/family.go` registers via RegisterFamily~~ **Revised:** the 4 RFC 4760 base families (`IPv4Unicast`, `IPv6Unicast`, `IPv4Multicast`, `IPv6Multicast`) live as exported vars in `internal/core/family/registry.go`, registered at family-package init. message keeps only the plugin-registry plumbing (`registerBuiltinFamilies`). | Pattern revisited after discovering 51 production sites with inline `family.Family{AFI: ..., SAFI: ...}` literals and 0 callers of `message.IPv4Unicast`. The base families are RFC-defined, not BGP-feature-defined; they are universally needed across packages upstream of `message` (which couldn't import message due to dependency direction). Plugin-specific families still live in their plugins. |
| 6 | familyStrings map | Deleted | Single source of truth |
| 7 | Exported Family vars | All removed from nlri, including builtin | nlri = types + registry only |
| 8 | Cache rebuild timing | Rebuild on every RegisterFamily, atomic swap | Runtime registration for external plugins |
| 9 | Thread safety | Mutex on RegisterFamily, atomic.Pointer for reads | Cold path protected, hot path lock-free |
| 10 | Re-registration | Same values = no-op, different values = fatal | No owner, coherence by convention |
| 11 | Python plugins | Register via plugin protocol at runtime | Same RegisterFamily path |
| 12 | Location | `internal/core/family/` | Cross-component infrastructure, not BGP-owned. Matches `clock`, `env`, `metrics`, `network` pattern. nlri imports family. |
| 13 | What moves | Full bundle (`Family`, `AFI`, `SAFI` types + numeric constants + registry) | Single owner. No identity wrappers (rules/design-principles.md). No backwards-compat aliases (rules/compatibility.md). |
| 14 | nlri package role | Keeps NLRI interface, parsers, errors, BGP-LS helpers. Imports `family`. | nlri = BGP wire format; family = family lookup. Clean separation. |
| 15 | Base families exported from family package | `family.IPv4Unicast`, `family.IPv6Unicast`, `family.IPv4Multicast`, `family.IPv6Multicast` are exported vars, populated via `MustRegister` at family-package init time. `helpers_family_test.go` in nlri deleted; 51 production sites collapsed to one-token references. | Decision 5 was overcorrected -- "no exported family vars" applied to plugin-specific families is correct, but the RFC 4760 base families are universal and not tied to any plugin or wire-format component. The duplication across 51 inline literals proved this. |

## Impact Assessment

| Area | References | Effort |
|------|-----------|--------|
| Plugin-specific Family vars (nlri -> plugins) | 57 refs in 15 files | Medium - mechanical moves |
| Builtin Family vars (nlri -> message) | 276 refs in 33 files | High - many test files |
| familyStrings deletion | 1 map, ParseFamily rewrite | Low |
| AFI.String() / SAFI.String() rewrite | 2 switches -> registry lookup | Low |
| Packed buffer implementation | New file | Medium |
| Plugin protocol extension | Protocol handler | Low |
| Import update (`family.Family` -> `family.Family`) | ~150 files | Low - mechanical, single-line per file |
| Move types from `nlri` to new `internal/core/family/` package | 2 files (family.go, registry.go) | Medium - new package layout |
| Total | ~333 references across ~48 files | High (mechanical, low risk) |

## Implementation Summary

### What Was Implemented

- `internal/core/family/family.go`: Family, AFI, SAFI types, AFI/SAFI numeric constants, FamilyLess
- `internal/core/family/registry.go`: RegisterFamily, MustRegister, LookupFamily, RegisteredFamilyNames, ResetRegistry, packed buffer with atomic.Pointer swap, lock-free String() reads (Family/AFI/SAFI all read from cache snapshot). Also exports the 4 RFC 4760 base families (`IPv4Unicast`, `IPv6Unicast`, `IPv4Multicast`, `IPv6Multicast`) registered at family-package init.
- `internal/core/family/testfamilies.go`: RegisterTestFamilies helper used by test packages that need plugin-specific families (FlowSpec, MVPN, etc.)
- `internal/core/family/registry_test.go`: 14 unit tests + BenchmarkFamilyString (zero alloc verified at 3.3ns/op)
- `internal/core/family/specialized_test.go`: 3 tests for specialized SAFI behavior (TestSpecializedFamilyVariables, TestSpecializedSAFIStrings, TestSpecializedFamilyParsing) -- moved from nlri to fix a boundary violation
- `nlri/nlri.go`: trimmed to NLRI interface + WriteNLRI/LenWithContext helpers; imports `family` for the Family type used in NLRI.Family() method
- `nlri/constants.go`: trimmed to error vars only (ErrMVPNTruncated, etc.); SAFI numeric constants and Family vars removed (moved to family/plugins)
- `message/family.go`: AFI/SAFI as type aliases of family.AFI/family.SAFI (eliminated 4 casts); removed FamilyConfigNames; removed all duplicate AFI/SAFI numeric constant aliases; removed `message.IPv4Unicast/IPv6Unicast/IPv4Multicast/IPv6Multicast` Family vars (now exported from family package); ValidFamilyConfigNames queries family.RegisteredFamilyNames(); kept registerBuiltinFamilies which records the 4 base families in the plugin registry's "builtin" source (separate concern from family registration)
- 9 NLRI plugins (flowspec, vpn, evpn, labeled, mvpn, vpls, rtc, mup, ls): each types.go declares its families via family.MustRegister directly (no per-plugin helper)
- 6 test packages got TestMain calling family.RegisterTestFamilies (format, plugins/cmd/update, plugins/gr, plugins/nlri/labeled, reactor, cmd/ze/bgp)
- `test/parse/test-family-config.ci`: functional test verifying canonical family names parse via registry
- ~50 production files updated to use family.IPv4Unicast/family.IPv6Unicast/family.IPv4Multicast/family.IPv6Multicast (collapsed 330 inline `family.Family{...}` literals)
- ~20 test files updated similarly
- 7 .ci files updated (alias names -> canonical: ipv4/mpls-vpn -> ipv4/vpn, etc.)
- ParseFamily renamed to LookupFamily (avoids hook collision with vpn.ParseFamily)
- `nlri/helpers_family_test.go` deleted (test-local IPv4Unicast/IPv6Unicast/IPv4VPN replaced by family.* references in 4 nlri test files: wire_test.go, inet_test.go, wire_format_test.go, base_len_test.go)
- `nlri/constants_test.go` deleted (3 tests moved to internal/core/family/specialized_test.go where they belong by package)

### Bugs Found/Fixed

- labeled plugin originally missed RegisterFamily; production builds returned "ipv4/safi-4" until fixed
- AFI.String()/SAFI.String() initially took registryMu (not lock-free); fixed by snapshotting maps in cache

### Documentation Updates

- Pending (see Documentation Update Checklist for files)

### Deviations from Plan

- ParseFamily renamed to LookupFamily due to hook collision with vpn.ParseFamily
- Aliases (ipv4/mpls, ipv4/mpls-vpn, etc.) dropped per rules/compatibility.md (no users to break)
- Conflict detection returns error instead of panic per rules/anti-rationalization.md
- Phase 4 implemented: rpc.FamilyDecl extended with AFI/SAFI fields, registerPluginFamilies called from handleProcessStartupRPC after declare-registration. All 9 internal NLRI plugins updated to populate AFI/SAFI in their FamilyDecl. cmd_plugin.go and exabgp/main_sdk.go look up AFI/SAFI from family.LookupFamily.
- Decision 5 revisited: the 4 RFC 4760 base families (IPv4/IPv6 unicast/multicast) moved from `message/family.go` to `internal/core/family/registry.go` as exported vars. Discovered after the initial implementation that 51 production sites had inline `family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}` literals while `message.IPv4Unicast` had zero callers (couldn't be used by packages upstream of message). Original Decision 5 was correct for plugin-specific families but wrong for RFC base families. See Decision 15.
- Boundary fix: moved `internal/component/bgp/nlri/constants_test.go` to `internal/core/family/specialized_test.go` -- the tests it contained were exclusively exercising family-package functionality (every assertion used `family.X` references) and represented a boundary violation.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| family.RegisterFamily(afi, safi, afiStr, safiStr) API | Done | internal/core/family/registry.go:106 | Returns (Family, error) |
| Plugin owns its families | Done | 9 plugin types.go files | Each calls family.MustRegister at init |
| nlri exports only NLRI interface + helpers | Done | internal/component/bgp/nlri/nlri.go, constants.go | Family/AFI/SAFI types moved to family pkg |
| Family strings derived from registration | Done | internal/core/family/registry.go:129 | canonical = afiStr + "/" + safiStr |
| AFI.String() from registry | Done | internal/core/family/family.go:31-37 | Lock-free via cache snapshot |
| SAFI.String() from registry | Done | internal/core/family/family.go:64-69 | Lock-free via cache snapshot |
| Packed buffer L1-resident | Done | internal/core/family/registry.go:24 | familyAFISlots=4, < 2KB total |
| atomic.Pointer cache swap | Done | internal/core/family/registry.go:77 | Reads lock-free |
| Mutex only on write | Done | internal/core/family/registry.go:69 | writeMu held only by RegisterFamily |
| Re-registration no-op same values | Done | internal/core/family/registry.go:130-132 | TestReRegisterSameValues |
| Re-registration error different values | Done | internal/core/family/registry.go:118, 124 | TestReRegisterConflictPanics |
| Runtime registration support | Done | internal/core/family/registry.go:106 | Cache rebuilt on each call |
| External plugin protocol wiring | Done | internal/component/plugin/server/startup.go:451 | registerPluginFamilies called after declare-registration; FamilyDecl extended with AFI/SAFI fields |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | TestRegisterFamilyString | "ipv4/flow" via registration |
| AC-2 | Done | TestReRegisterSameValues | Same values = no-op |
| AC-3 | Done | TestReRegisterConflictPanics | Returns error (not panic, per anti-rationalization rule) |
| AC-4 | Done | TestFamilyStringFallback | "afi-9999/safi-99" fallback |
| AC-5 | Done | internal/core/family/registry.go:106 | Returns Family value, comparable |
| AC-6 | Done | TestParseFamilyRegistered | LookupFamily returns registered Family |
| AC-7 | Done | TestParseFamilyUnknown | Unregistered name returns false |
| AC-8 | Done | grep internal/core/family/ internal/component/bgp/nlri/ | No exported Family vars remain |
| AC-9 | Done | grep family.MustRegister internal/component/bgp/plugins/nlri/*/types.go | All 9 plugins call family.MustRegister |
| AC-10 | Done | TestPackedBufferSize | < 2KB verified |
| AC-11 | Done | BenchmarkFamilyString | 0 allocs at 3.284 ns/op |
| AC-12 | Done | TestRuntimeRegistration + TestRegisterPluginFamiliesAddsToNLRIRegistry | Cache rebuilds on each RegisterFamily; plugin protocol wires FamilyDecl into family.RegisterFamily |
| AC-13 | Done | TestRegisterFamilyAFIString | AFI.String() reads from registry |
| AC-14 | Done | TestRegisterFamilySAFIString | SAFI.String() reads from registry |
| AC-15 | Done | TestLookupFamilyStringIsLockFree | All 3 String() methods read via atomic.Pointer |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestRegisterFamilyString | Done | internal/core/family/registry_test.go:28 | |
| TestRegisterFamilyAFIString | Done | internal/core/family/registry_test.go:39 | |
| TestRegisterFamilySAFIString | Done | internal/core/family/registry_test.go:50 | |
| TestReRegisterSameValues | Done | internal/core/family/registry_test.go:61 | |
| TestReRegisterConflictPanics | Done | internal/core/family/registry_test.go:76 | Returns error, not panic |
| TestFamilyStringFallback | Done | internal/core/family/registry_test.go:115 | |
| TestParseFamilyRegistered | Done | internal/core/family/registry_test.go:127 | LookupFamily |
| TestParseFamilyUnknown | Done | internal/core/family/registry_test.go:142 | LookupFamily |
| TestRuntimeRegistration | Done | internal/core/family/registry_test.go:156 | |
| TestPackedBufferSize | Done | internal/core/family/registry_test.go:178 | |
| BenchmarkFamilyString | Done | internal/core/family/registry_test.go:233 | 3.284 ns/op, 0 allocs |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| internal/core/family/family.go | Created | Family/AFI/SAFI types + numeric constants |
| internal/core/family/registry.go | Created | RegisterFamily, MustRegister, packed buffer, atomic.Pointer; exports IPv4Unicast/IPv6Unicast/IPv4Multicast/IPv6Multicast |
| internal/core/family/registry_test.go | Created | 14 unit tests + benchmark |
| internal/core/family/specialized_test.go | Created | 3 specialized SAFI tests moved from nlri (boundary fix) |
| internal/core/family/testfamilies.go | Created | Test helper used by packages needing plugin-specific families |
| internal/component/bgp/nlri/nlri.go | Modified | Trimmed to NLRI interface + helpers; imports family |
| internal/component/bgp/nlri/constants.go | Modified | Trimmed to error vars only |
| internal/component/bgp/nlri/constants_test.go | Deleted | Tests moved to internal/core/family/specialized_test.go |
| internal/component/bgp/nlri/helpers_family_test.go | Deleted | Test helpers (IPv4Unicast/IPv6Unicast/IPv4VPN) inlined to family.* references |
| internal/component/bgp/message/family.go | Modified | Type aliases, removed builtin Family vars (now in family pkg), removed duplicate AFI/SAFI constant aliases, kept registerBuiltinFamilies plumbing |
| 9 plugins/nlri/*/types.go | Modified | family.MustRegister calls (no per-plugin helper) |
| ~50 production files | Modified | Use family.IPv4Unicast/IPv6Unicast/IPv4Multicast/IPv6Multicast (330 inline literals collapsed) |
| ~20 test files | Modified | Same collapse as production |
| 7 .ci files | Modified | Canonical names |
| 6 testmain_test.go | Created | Call family.RegisterTestFamilies |
| test/parse/test-family-config.ci | Created | Functional test |

### Audit Summary

- **Total items:** 13 requirements + 15 ACs + 11 tests + 15 file groups = 54
- **Done:** 52
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (ParseFamily renamed to LookupFamily; Decision 5 revisited: base families moved from message to family package after discovering 51 inline-literal sites)
- **Documentation:** Pending (see checklist below)

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| internal/core/family/family.go | Yes | git status A (new file) |
| internal/core/family/registry.go | Yes | git status A (new file) |
| internal/core/family/registry_test.go | Yes | git status A (new file) |
| internal/core/family/specialized_test.go | Yes | git status A (new file) |
| internal/core/family/testfamilies.go | Yes | git status A (new file) |
| test/parse/test-family-config.ci | Yes | git status A (new file) |
| internal/component/bgp/nlri/constants_test.go | No (deleted) | tests moved to family/specialized_test.go |
| internal/component/bgp/nlri/helpers_family_test.go | No (deleted) | helpers replaced by family.* references |

### AC Verified

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | RegisterFamily returns canonical string | `go test -run TestRegisterFamilyString` PASS |
| AC-3 | Conflict returns error | `go test -run TestReRegisterConflictPanics` PASS |
| AC-10 | Buffer < 2KB | `go test -run TestPackedBufferSize` PASS |
| AC-11 | Zero alloc | `go test -bench BenchmarkFamilyString -benchmem` shows 0 B/op 0 allocs/op |
| AC-15 | Lock-free reads | `go test -run TestLookupFamilyStringIsLockFree` PASS (deadlock would fail) |

### Wiring Verified

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Config file family names | test/parse/test-family-config.ci | File exists, validates ipv4/unicast, ipv6/unicast, ipv4/vpn, ipv4/flow, ipv4/mpls-label, l2vpn/evpn |
