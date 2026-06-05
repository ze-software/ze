# Spec: typed-peer-selector

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 8/12 |
| Updated | 2026-06-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/enum-over-string.md` - typed numeric over string
4. `ai/rules/plugin-design.md` - cross-boundary value types
5. `internal/core/selector/selector.go` - existing typed selector package
6. `internal/component/bgp/types/reactor.go` - BGPReactor interface
7. `pkg/plugin/sdk/sdk_engine.go` - SDK dispatch methods
8. `pkg/plugin/rpc/bridge.go` - DirectBridge handler types

## Task

Replace internal magic string peer selector encodings with the existing typed `*selector.Selector` model. Push the parse boundary outward to CLI/RPC entry points. Internal dispatch and route update paths pass `*selector.Selector` instead of `string` where both caller and callee are in-process Go code.

Scope:
- BGPReactor interface methods: `peerSelector string` to `sel *selector.Selector`
- SDK typed dispatch: add `*Sel` variant methods for in-process plugins via DirectBridge
- Fix SoftClearPeer to use typed selector dispatch (bug: currently only supports IP/glob)
- Consolidate duplicate `parseSel`/`parseReactorSel` into `selector.ParseOrAll`
- Unify the fourth resolver (`filterPeersBySelectorValue`) to use the selector package
- Tests for ambiguous inputs (peer names starting with `!`, `as`, `*`)

## Required Reading

### Architecture Docs
- [ ] `ai/rules/enum-over-string.md` - typed numeric identity on hot paths, strings only at boundaries
  → Constraint: parse string once at boundary (config load, CLI parse, JSON unmarshal), pass typed value everywhere internally
  → Constraint: `String()` is for diagnostics, never comparison
- [ ] `ai/rules/plugin-design.md` - cross-boundary value types, proximity principle
  → Constraint: payloads crossing plugin/component boundaries MUST be self-contained value types
  → Decision: `*selector.Selector` contains only `netip.Addr`, `[]netip.Addr`, `uint32`, `string`, `Kind` (uint8) -- all value types, no cross-component pointers; safe to pass across plugin boundaries
- [ ] `ai/rules/wiring-completeness.md` - every exported function must have a non-test caller
  → Constraint: new `ParseOrAll()` and typed SDK methods must have production callers
- [ ] `docs/architecture/api/process-protocol.md` - plugin communication protocol
  → Constraint: external plugins use newline-framed YANG RPCs with string payloads; typed selectors cannot cross the wire
  → Decision: DirectBridge (in-process) carries `*selector.Selector`; JSON RPC path stays string-based

### Learned Summaries
- [ ] `plan/learned/411-peer-selector-asn.md` - three independent selector resolvers must stay in sync
  → Constraint: four resolvers exist: (1) `getMatchingPeersSel` (reactor), (2) `filterPeersBySelectorValue` (peer commands), (3) `SoftClearPeer` (route refresh), (4) `parseSel` (RIB/adj-rib-in show commands)
  → Constraint: `SoftClearPeer` uses `ipGlobMatch` directly, does not support name/ASN selectors (pre-existing bug)
- [ ] `plan/learned/830-typed-inter-plugin-dispatch.md` - `DispatchCommandArgs` is the typed boundary
  → Decision: `DispatchCommandArgs` was introduced to bypass tokenizer; peer is string at this layer for external compat
  → Constraint: legacy `aaa.Authorizer` implementations need stable string arg boundaries (canonical fallback)
  → Constraint: adj-rib-in selector compatibility: `show adj-rib-in <peer>` delivers selector through args[0], not peer

**Key insights:**
- Four independent selector resolvers exist today; all must be aligned
- The `selector` package already handles all seven kinds (All, Addr, Exclude, Addrs, Name, ASN, Glob)
- `SoftClearPeer` is buggy: only supports IP/glob, not name/ASN/exclude
- GR calls `DispatchCommandArgs` (SDK boundary); RR/RS call `UpdateRoute` (SDK boundary)
- Both SDK paths have DirectBridge fast paths that bypass JSON serialization
- External plugins cannot receive `*selector.Selector` over the wire; typed path is DirectBridge-only
- Authorization must still see string form for legacy authorizer compatibility
- `ForwardUpdate` already takes `*selector.Selector` -- proof pattern works in BGPReactor

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/selector/selector.go` (251L) - typed Selector with Kind enum (7 kinds), Parse(), Matches(), String(), constructors
  → Constraint: round-trip property must be preserved: Parse(sel.String()) produces equivalent selector
  → Constraint: Matches() returns false for KindName/KindASN/KindGlob (IP-only matching)
- [ ] `internal/component/bgp/types/reactor.go` (80L) - BGPReactor interface with 8 methods taking `peerSelector string`, ForwardUpdate already typed
  → Constraint: interface is internal; implementations in reactor_api*.go and mock in update_text_test.go
- [ ] `internal/component/bgp/reactor/reactor_api.go` (1186L) - `parseReactorSel()` parses string->typed; `getMatchingPeersSel()` dispatches on Kind
  → Constraint: `SoftClearPeer` (line 234) uses raw `ipGlobMatch` loop instead of `getMatchingPeersSel`
  → Constraint: `getMatchingPeers(selectorStr)` calls `parseReactorSel` then `getMatchingPeersSel` -- after change, callers pass typed directly
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `AnnounceNLRIBatch`, `WithdrawNLRIBatch`, `SendRoutes` each call `parseReactorSel` internally
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `AnnounceEOR`, `SendRefresh`, `SendBoRR`, `SendEoRR` each call `parseReactorSel` internally
- [ ] `internal/component/bgp/plugins/gr/gr.go` (597L) - constructs `selector.ExcludeAddr(addr).String()` (line 134) then dispatches via `sdk.DispatchCommandArgs`
  → Constraint: GR uses `dispatchCommand` helper which calls `sdk.DispatchCommandArgs(ctx, command, args, "")` -- peer selector is in args[0], not the peer param
- [ ] `internal/component/bgp/plugins/rr/rr.go` - constructs `selector.ExcludeAddr(addr).String()` (line 285) then calls `plugin.UpdateRoute()`
  → Constraint: RR `updateRoute` method calls `sdk.UpdateRoute(ctx, peerSelector, command)` -- selector is the peerSelector param
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` - constructs `selector.ExcludeAddr(addr).String()` (line 116) then calls `rs.updateRoute()`
  → Constraint: RS `updateRoute` method calls `sdk.UpdateRoute(ctx, peerSelector, command)` -- same as RR
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - local `parseSel()` (line 557-565), duplicates `parseReactorSel()` logic
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - local `parseSel()` (line 284-292), duplicates same logic
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` (line 57-99) - `filterPeersBySelectorValue()` re-implements selector logic ad-hoc with IP, name, ASN branches
  → Constraint: does not support exclude, glob, or multi-addr selectors (silent no-match)
- [ ] `pkg/plugin/sdk/sdk_engine.go` - `DispatchCommandArgs(ctx, command, args, peer string)`, `UpdateRoute(ctx, peerSelector, command string)`, both with DirectBridge fast path
- [ ] `pkg/plugin/rpc/bridge.go` - `DispatchCommandArgsHandler func(command string, args []string, peer string)`, DirectBridge typed handler
- [ ] `internal/component/plugin/server/dispatch.go` (line 788-793) - registers bridge handlers; `dispatchCommandArgs` (line 652) creates CommandContext with `Peer: peer`
- [ ] `internal/component/bgp/transaction/commit_manager.go` - `peerSelector string` field (line 33) in Transaction struct

**Behavior to preserve:**
- CLI peer selector syntax unchanged: `*`, `<ip>`, `!<ip>`, `<ip>,<ip>`, `as<N>`, `<glob>`, `<name>`
- External plugin RPC wire protocol unchanged (string-based JSON payloads)
- Authorization receives canonical string form for legacy authorizers via `aaa.CanonicalCommand`
- `show adj-rib-in <peer>` compatibility: selector through args[0]
- Round-trip: `Parse(sel.String())` produces equivalent selector for all kinds
- GR `dispatchCommand` peer selector position: in args, not peer param (keeps working for commands like `clear bgp rib out`)

**Behavior to change:**
- BGPReactor interface methods accept `*selector.Selector` instead of `peerSelector string`
- SDK gains `UpdateRouteSel` and `DispatchCommandArgsSel` methods; DirectBridge gains matching handlers
- `SoftClearPeer` uses `getMatchingPeersSel()` -- fixes name/ASN/exclude support
- GR/RR/RS plugins call `*Sel` variants with typed selector instead of stringifying
- `filterPeersBySelectorValue` uses `selector.Parse()` instead of ad-hoc logic
- Duplicate `parseSel`/`parseReactorSel` replaced by `selector.ParseOrAll()`
- Transaction `peerSelector` field changes from `string` to `*selector.Selector`

## Data Flow (MANDATORY)

### Entry Point
- **CLI boundary**: user types `peer 10.0.0.1 list` -- dispatcher extracts peer selector string, calls `selector.Parse()` at entry
- **External plugin RPC**: JSON wire `{"peer": "!10.0.0.1"}` -- string arrives, parsed at handler entry
- **In-process plugin**: GR/RR/RS construct `*selector.Selector` typed from the start (e.g., `selector.ExcludeAddr(addr)`)

### Transformation Path
1. **Boundary parse**: CLI/RPC handler calls `selector.Parse(s)` or `selector.ParseOrAll(s)` to produce `*selector.Selector`
2. **Typed propagation**: `*selector.Selector` flows through BGPReactor interface, SDK typed dispatch, DirectBridge
3. **Reactor resolution**: `getMatchingPeersSel(sel)` dispatches on `sel.SelectorKind()` to resolve matching `[]*Peer`
4. **External serialization** (when needed): `sel.String()` produces canonical string for legacy auth or wire protocol

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI dispatcher -> BGPReactor | `*selector.Selector` passed directly | [ ] |
| In-process plugin -> SDK DirectBridge | `*selector.Selector` via `*Sel` typed handler | [ ] |
| External plugin -> SDK JSON RPC | `peerSelector string` in JSON, parsed at handler entry | [ ] |
| SDK -> authorization | `sel.String()` for canonical form if legacy authorizer | [ ] |
| Plugin -> RIB show commands | `selector.ParseOrAll(s)` at command handler entry | [ ] |

### Integration Points
- `selector.ParseOrAll(s)` replaces `parseSel()` and `parseReactorSel()` -- single parse-or-default function
- `BGPReactor` interface methods -- widest API surface change (8 methods)
- `DirectBridge` typed handlers -- new `*Sel` variants alongside existing string ones
- `filterPeersBySelectorValue` -- replaced by `selector.Parse()` + kind dispatch

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (`selector` package imports only `netip`, `stringsx`, `textbuf`)
- [ ] No duplicated functionality (consolidates four resolvers into one pattern)
- [ ] Zero-copy preserved where applicable (selector passed by pointer)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add `*Sel` variant methods to SDK | Change existing signatures + add string wrappers | Zero risk to external plugins, matches ForwardCached pattern |
| BGPReactor takes `*selector.Selector` (breaking) | Keep string, parse inside each method | This is internal interface; eliminates parse-stringify-reparse; ForwardUpdate already proves the pattern |
| `ParseOrAll` treats errors as All | Return error, let caller decide | Matches behavior of all four existing resolvers; changing would alter existing fail-open semantics |
| DirectBridge carries typed selector | Only change BGPReactor, stringify at SDK boundary | User chose wider scope; eliminates the last stringify in the in-process path |

## Known Limitations
- External plugins continue to use string selectors over JSON RPC (cannot pass Go types over wire)
- `ParseOrAll` treats parse errors as "all peers" (matches current behavior of all four resolvers)
- Peer names starting with `!` cannot be expressed in string syntax (documented constraint, not a bug)
- GR plugin's `dispatchCommand` helper passes selector in args[0] as string (the target command handler parses it); typed path only applies to the exclude selector in `onLLGREntryDone`

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| GR plugin constructs `ExcludeAddr` | -> | SDK `DispatchCommandArgsSel` carries `*selector.Selector` | `TestGRExcludeUsesTypedSelector` |
| RR plugin constructs `ExcludeAddr` | -> | `UpdateRouteSel` carries typed selector | `TestRRWithdrawalUsesTypedSelector` |
| RS plugin constructs `ExcludeAddr` | -> | `UpdateRouteSel` carries typed selector | `TestRSWithdrawalUsesTypedSelector` |
| `SoftClearPeer(selector.ASN(65000))` | -> | `getMatchingPeersSel` dispatches on KindASN | `TestSoftClearPeerASN` |
| `peer as65000 list` CLI | -> | `filterPeersBySelectorValue` uses `selector.Parse` | `TestPeerListASNSelector` |
| `selector.ParseOrAll("")` | -> | returns `All()` | `TestParseOrAllEmpty` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BGPReactor interface methods | All 8 methods take `*selector.Selector` instead of `peerSelector string` |
| AC-2 | GR/RR/RS plugins construct exclude selector | No `.String()` call before reaching reactor; typed through DirectBridge `*Sel` methods |
| AC-3 | `SoftClearPeer` called with `selector.ASN(65000)` | Matches all peers with ASN 65000 (currently broken) |
| AC-4 | `SoftClearPeer` called with `selector.PeerName("core-rr")` | Matches peer by name (currently broken) |
| AC-5 | External plugin sends `dispatch-command-args` with peer selector string | String parsed at handler entry, typed selector flows internally |
| AC-6 | `filterPeersBySelectorValue` called with `"as65000"` | Uses `selector.Parse`, returns matching peers (currently works but via ad-hoc code) |
| AC-7 | `selector.Parse("!router1")` | Returns error (not valid IP); test documents the constraint |
| AC-8 | `selector.ParseOrAll("")` and `selector.ParseOrAll("*")` | Both return `KindAll` |
| AC-9 | Authorization handler on typed dispatch path | Receives canonical string form via `sel.String()` for legacy authorizer |
| AC-10 | Duplicate `parseSel`/`parseReactorSel` helpers | Replaced by `selector.ParseOrAll`; `grep -rn 'func parseSel\|func parseReactorSel' internal/` returns 0 |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseOrAllEmpty` | `internal/core/selector/selector_test.go` | `ParseOrAll("")` returns KindAll | |
| `TestParseOrAllStar` | `internal/core/selector/selector_test.go` | `ParseOrAll("*")` returns KindAll | |
| `TestParseOrAllValid` | `internal/core/selector/selector_test.go` | `ParseOrAll("10.0.0.1")` returns KindAddr | |
| `TestParseOrAllError` | `internal/core/selector/selector_test.go` | `ParseOrAll("!invalid")` returns KindAll (error falls to all) | |
| `TestParseExclamationName` | `internal/core/selector/selector_test.go` | `Parse("!router1")` returns error | |
| `TestSoftClearPeerASN` | `internal/component/bgp/reactor/reactor_api_test.go` | SoftClearPeer matches peers by ASN | |
| `TestSoftClearPeerName` | `internal/component/bgp/reactor/reactor_api_test.go` | SoftClearPeer matches peers by name | |
| `TestFilterPeersSelectorParse` | `internal/component/bgp/plugins/cmd/peer/peer_test.go` | filterPeersBySelectorValue uses selector.Parse for all kinds | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ASN in selector | 1-4294967295 | as4294967295 | as0 (parsed as name) | as4294967296 (parsed as name) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-peer-selector-typed` | `test/plugin/peer-selector-typed.ci` | Typed selector flows from plugin through reactor for exclude, ASN, name kinds | |

### Interop Tests
N/A -- no wire protocol change. Peer selector syntax is unchanged externally.

### Future
- None

## Files to Modify
- `internal/core/selector/selector.go` - add `ParseOrAll()`
- `internal/core/selector/selector_test.go` - tests for `ParseOrAll`, ambiguity tests
- `internal/component/bgp/types/reactor.go` - change 8 methods from `peerSelector string` to `sel *selector.Selector`
- `internal/component/bgp/reactor/reactor_api.go` - update `SoftClearPeer` to use `getMatchingPeersSel`, remove `parseReactorSel`, update `getMatchingPeers`
- `internal/component/bgp/reactor/reactor_api_batch.go` - accept `*selector.Selector`, remove internal `parseReactorSel` calls
- `internal/component/bgp/reactor/reactor_api_forward.go` - accept `*selector.Selector`, remove internal `parseReactorSel` calls
- `internal/component/bgp/plugins/gr/gr.go` - use `DispatchCommandArgsSel` for exclude selector path
- `internal/component/bgp/plugins/rr/rr.go` - use `UpdateRouteSel` for exclude selector path
- `internal/component/bgp/plugins/rs/server.go` - `updateRoute` gains typed variant, calls `UpdateRouteSel`
- `internal/component/bgp/plugins/rs/server_handlers.go` - pass typed selector
- `internal/component/bgp/plugins/rib/rib_commands.go` - replace `parseSel` with `selector.ParseOrAll`
- `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - replace `parseSel` with `selector.ParseOrAll`
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - accept `*selector.Selector` where callers have it
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - accept `*selector.Selector` where callers have it
- `internal/component/bgp/plugins/cmd/peer/peer.go` - replace `filterPeersBySelectorValue` ad-hoc logic with `selector.Parse`
- `pkg/plugin/sdk/sdk_engine.go` - add `UpdateRouteSel`, `DispatchCommandArgsSel`
- `pkg/plugin/rpc/bridge.go` - add `UpdateRouteSelHandler`, `DispatchCommandArgsSelHandler`, Set/Has/Call methods
- `internal/component/plugin/server/dispatch.go` - register typed selector bridge handlers
- `internal/component/bgp/transaction/commit_manager.go` - change `peerSelector string` to `*selector.Selector`
- `internal/component/bgp/plugins/cmd/update/update_text_test.go` - update mock BGPReactor signatures
- `docs/architecture/core-design.md` - update peer selector section

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | No config/RPC change |
| YANG validation constraints | No | |
| YANG custom validators | No | |
| CLI commands/flags | No | CLI syntax unchanged |
| CLI grammar | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | Yes | `test/plugin/peer-selector-typed.ci` |
| Pipe completeness | No | No new output commands |
| Env var registration | No | |
| Doctor check | No | |
| Prometheus counters | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Internal refactor only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | External API unchanged |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` -- document typed selector on DirectBridge |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` -- peer selector section |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered plugin/event/command changed? | No | |
| 16 | Source anchors referencing changed files? | Yes | Grep `docs/` for source anchors pointing at reactor_api.go, bridge.go, sdk_engine.go |
| 17 | Existing examples for this area? | No | |

## Files to Create
- `test/plugin/peer-selector-typed.ci` - functional test for typed selector propagation

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

1. **Phase: Wiring** -- add `ParseOrAll` to selector package, write failing tests
   - Tests: `TestParseOrAllEmpty`, `TestParseOrAllStar`, `TestParseOrAllValid`, `TestParseOrAllError`, `TestParseExclamationName`
   - Files: `internal/core/selector/selector.go`, `internal/core/selector/selector_test.go`
   - Verify: new tests pass; `ParseOrAll` is the public API for "parse with all-peers fallback"

2. **Phase: BGPReactor interface** -- change 8 interface methods from `string` to `*selector.Selector`
   - Tests: compile errors guide the mechanical change across all implementations
   - Files: `internal/component/bgp/types/reactor.go`, `reactor_api.go`, `reactor_api_batch.go`, `reactor_api_forward.go`
   - Verify: compiles; internal `parseReactorSel` calls removed from method bodies; existing tests pass

3. **Phase: Callers of BGPReactor** -- update all code that calls the changed interface methods
   - Tests: compile errors guide
   - Files: `transaction/commit_manager.go`, `update_text.go`, `update_text_test.go` (mock), any other callers found via LSP findReferences
   - Verify: compiles; callers pass `*selector.Selector` (parsed at their boundary)

4. **Phase: SoftClearPeer fix** -- rewrite to use `getMatchingPeersSel` instead of `ipGlobMatch`
   - Tests: `TestSoftClearPeerASN`, `TestSoftClearPeerName`
   - Files: `internal/component/bgp/reactor/reactor_api.go`
   - Verify: ASN and name selectors now work in SoftClearPeer

5. **Phase: SDK typed selector** -- add `*Sel` variant methods and DirectBridge handlers
   - Tests: basic unit tests for new methods
   - Files: `pkg/plugin/rpc/bridge.go`, `pkg/plugin/sdk/sdk_engine.go`, `internal/component/plugin/server/dispatch.go`
   - Verify: `UpdateRouteSel` and `DispatchCommandArgsSel` exist and are wired through DirectBridge

6. **Phase: Plugin callers** -- GR/RR/RS use `*Sel` variants
   - Tests: `TestGRExcludeUsesTypedSelector`, `TestRRWithdrawalUsesTypedSelector`, `TestRSWithdrawalUsesTypedSelector`
   - Files: `gr/gr.go`, `rr/rr.go`, `rs/server.go`, `rs/server_handlers.go`
   - Verify: no `.String()` between typed construction and SDK call on the in-process path

7. **Phase: Consolidate parseSel** -- replace duplicates with `selector.ParseOrAll`
   - Tests: existing show command tests continue to pass
   - Files: `rib/rib_commands.go`, `adj_rib_in/rib_commands.go`, `rib/rib_pipeline.go`, `rib/rib_pipeline_best.go`
   - Verify: `grep -rn 'func parseSel\|func parseReactorSel' internal/` returns 0

8. **Phase: Unify filterPeersBySelectorValue** -- use `selector.Parse` + kind dispatch
   - Tests: `TestFilterPeersSelectorParse`, `TestPeerListASNSelector`
   - Files: `plugins/cmd/peer/peer.go`
   - Verify: peer commands handle all selector kinds consistently; exclude/glob/multi-addr now supported

9. **Functional tests** -- create `.ci` test for end-to-end typed selector flow
10. **Documentation** -- update `docs/architecture/core-design.md` peer selector section, `process-protocol.md` DirectBridge section
11. **Full verification** -- `make ze-verify`
12. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has implementation with file:line |
| Correctness | `SoftClearPeer` matches same peers as before for IP/glob, plus name/ASN/exclude |
| Naming | `ParseOrAll` follows selector package conventions; `*Sel` suffix on SDK methods |
| Data flow | String parsing happens only at CLI/RPC boundary, never mid-pipeline |
| No layering | `parseSel`, `parseReactorSel` fully deleted, not dual-pathed alongside `ParseOrAll` |
| Authorization | Legacy authorizer still receives `sel.String()` canonical form via `aaa.CanonicalCommand` |
| Four resolvers aligned | All four produce same results for `*`, `10.0.0.1`, `!10.0.0.1`, `as65000`, `peer-name`, `10.*.*.*` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `selector.ParseOrAll` exists | `grep -n 'func ParseOrAll' internal/core/selector/selector.go` |
| BGPReactor methods take `*selector.Selector` | `grep 'peerSelector string' internal/component/bgp/types/reactor.go` returns 0 |
| No duplicate parseSel | `grep -rn 'func parseSel\|func parseReactorSel' internal/` returns 0 |
| SoftClearPeer uses getMatchingPeersSel | `grep 'ipGlobMatch' internal/component/bgp/reactor/reactor_api.go` returns 0 within SoftClearPeer |
| SDK `*Sel` methods exist | `grep 'UpdateRouteSel\|DispatchCommandArgsSel' pkg/plugin/sdk/sdk_engine.go` |
| DirectBridge typed selector handlers | `grep 'selector.Selector' pkg/plugin/rpc/bridge.go` shows handler types |
| filterPeersBySelectorValue uses selector.Parse | `grep 'selector.Parse' internal/component/bgp/plugins/cmd/peer/peer.go` |
| Functional test exists | `ls test/plugin/peer-selector-typed.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `selector.Parse` already validates; `ParseOrAll` treats errors as "all peers" -- verify this is safe (matches existing behavior) |
| Authorization bypass | Typed `*Sel` path must still pass through authorization; verify `isAuthorizedCommandArgs` called with `sel.String()` |
| Selector injection | Verify that typed selector cannot be constructed to bypass exclusion (KindExclude with zero-value IP should exclude nothing, not everything) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## Core Insight

## RFC Documentation

N/A -- no RFC protocol changes.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Internal paths use typed selector | Unit test | TestGRExcludeUsesTypedSelector, TestRRWithdrawalUsesTypedSelector |
| SoftClearPeer supports all selector kinds | Unit test | TestSoftClearPeerASN, TestSoftClearPeerName |
| No duplicate parseSel helpers | Grep | `grep -rn 'func parseSel' internal/` returns 0 |
| Four resolvers agree | Unit test | TestFilterPeersSelectorParse |
| Ambiguous inputs documented | Unit test | TestParseExclamationName |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs updated
- [ ] Critical Review passes

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
- [ ] Interop tests N/A (no wire protocol change)
- [ ] Goal Validation table filled

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Commit A: code + tests + docs + spec + learned summary
- [ ] Commit B: git rm spec
