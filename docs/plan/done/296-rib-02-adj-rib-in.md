# Spec: rib-02 — Adj-RIB-In Storage Plugin + Shared BGP Types

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/bgp-rib/rib.go`, `rib_commands.go`, `event.go`, `rib_nlri.go`
4. `docs/plan/spec-rib-01-dispatch-command.md` — prerequisite spec

## Task

Two deliverables:

**1. Shared BGP plugin types** — Extract common types from `bgp-rib` into `internal/plugin/bgp/shared/` so multiple BGP plugins can reuse them without code duplication. This is infrastructure code under `internal/plugin/` (not `internal/plugins/` which holds plugin implementations).

**2. bgp-adj-rib-in plugin** — New plugin that stores all received routes per source peer as raw hex wire bytes (from format=full events), enabling fast replay to reconnecting peers via `update hex` commands. Registers commands via `OnExecuteCommand` so other plugins can invoke replay through dispatch-command (spec rib-01).

Key differences from bgp-rib:
- Subscribes to `"update direction received"` (not "sent")
- Stores routes keyed by SOURCE peer (not destination peer)
- Stores raw hex wire bytes (AttrHex, NHopHex, NLRIHex) — NOT parsed Route structs or wire-byte pool handles
- Replay uses `update hex attr set <HEX> nhop set <HEX> nlri <FAM> add <HEX>` — avoids text formatting/parsing round-trip
- All address families stored (no `isSimplePrefixFamily` filter)
- Adds `"adj-rib-in replay"` command for incremental replay (invoked via dispatch-command from other plugins)
- Adds monotonic sequence index for incremental replay support

bgp-rib remains unchanged in this spec. bgp-rib is modified in spec rib-04.

**Depends on:** spec-rib-01-dispatch-command.md (dispatch-command RPC exists)
**Part of series:** rib-01 → rib-02 (this) → rib-03 → rib-04

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB storage internals
  → Decision: ~~bgp-adj-rib-in uses parsed Route structs~~ bgp-adj-rib-in uses raw hex wire bytes from format=full events, NOT parsed Route structs or wire-byte pool handles
- [ ] `docs/architecture/core-design.md` - plugin architecture
  → Constraint: plugins register via init() in registry, import rules apply
  → Constraint: `internal/plugin/` = infrastructure, `internal/plugins/` = implementations

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - BGP-4 specification
  → Constraint: Section 3.2 — Adj-RIBs-In = "unprocessed routing information advertised to the local BGP speaker by its peers"

**Key insights:**
- bgp-rib's ribOut already stores parsed Route structs with full attributes and has formatRouteCommand — this pattern moves to shared package (used by bgp-rib ribOut and adj-rib-in show command)
- bgp-rib's ribInPool uses wire bytes + pool handles — too complex for replay, wrong format for cross-plugin communication
- bgp-rib's Event struct already parses all attributes from JSON — moves to shared package
- bgp-rib subscribes to "sent" events only — ribInPool is empty in production. bgp-adj-rib-in subscribes to "received" events.
- bgp-rib's `isSimplePrefixFamily` filter skips VPN/EVPN/FlowSpec — bgp-adj-rib-in stores ALL families
- `internal/plugin/` is the correct home for shared BGP types (infrastructure, not implementation)
- **Raw hex replay avoids round-trip:** format=full events provide `raw.attributes` (path attrs hex without MP_REACH/UNREACH) and `raw.nlri[family]` (concatenated NLRI wire hex). Replay uses `update hex attr set ... nhop set ... nlri FAM add ...` which the engine decodes directly to wire bytes — skipping the entire text formatting + re-parsing pipeline
- **Next-hop conversion:** format=full doesn't include raw next-hop bytes separately (they're part of MP_REACH_NLRI which is excluded from raw.attributes). adj-rib-in converts the parsed next-hop IP to wire hex — one cheap `net.IP.To4()/To16()` + hex encode
- **Per-prefix keying from parsed data:** Parsed FamilyOps provide prefix/pathID for routeKey (add/del matching). Raw NLRI hex for individual prefixes comes from splitting `raw.nlri[family]` via `splitNLRIs()` (simple families) or `prefixToWire()` fallback

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/plugins/bgp-rib/register.go` - Registers as "bgp-rib", YANG "ze-rib"
- [x] `internal/plugins/bgp-rib/rib.go` - RIBManager, Route struct (72-88), handleSent (stores Route per dest peer), handleReceived (stores wire bytes via pool), replayRoutes, formatRouteCommand
- [x] `internal/plugins/bgp-rib/rib_commands.go` - Commands: rib show in/out, rib clear in/out, rib status. formatRouteCommand (202-267) builds "update text" from Route.
- [x] `internal/plugins/bgp-rib/event.go` - Event struct with parsed attributes (Origin, ASPath, MED, etc.), FamilyOperation with NextHop/Action/NLRIs, parseEvent with dual peer format handling
- [x] `internal/plugins/bgp-rib/rib_nlri.go` - parseNLRIValue, formatNLRIAsPrefix, routeKey, isSimplePrefixFamily

**Types to extract to shared package:**

| Type/Function | Current Location | Shared Package File |
|---------------|-----------------|---------------------|
| Route struct | `rib.go:70-88` | `route.go` |
| Event struct + parseEvent() | `event.go` | `event.go` |
| FamilyOperation, MessageInfo | `event.go` | `event.go` |
| formatRouteCommand() | `rib_commands.go:202-267` | `format.go` |
| parseNLRIValue(), routeKey() | `rib_nlri.go` | `nlri.go` |

**Types that stay in bgp-rib (not shared):**
- `isSimplePrefixFamily()` — only bgp-rib uses this for wire-byte pool filtering
- RIBManager, ribInPool, ribOut — bgp-rib internals
- storage/ subpackage — wire-byte pool storage (bgp-rib only)

**Behavior to preserve:**
- bgp-rib continues working identically after extraction (imports change, behavior doesn't)
- Route struct field names and types unchanged
- Event parsing handles both flat and nested peer formats
- formatRouteCommand output format unchanged ("update text ...")

**Behavior to change:**
- Route, Event, FamilyOperation, formatRouteCommand, parseNLRIValue, routeKey move to `internal/plugin/bgp/shared/`
- bgp-rib imports from shared package instead of defining locally
- New bgp-adj-rib-in plugin imports from shared package

## Data Flow (MANDATORY)

### Entry Point
- Engine sends "update direction received" JSON events to bgp-adj-rib-in
- Engine sends "state" events for peer up/down

### Transformation Path
1. Engine delivers UPDATE event (format=full) with parsed attributes + NLRI operations + raw hex bytes
2. `shared.ParseEvent()` → `shared.Event` with FamilyOps (parsed), RawAttributes (hex), RawNLRI (per-family hex), RawWithdrawn (per-family hex)
3. handleReceived: for each FamilyOperation with action="add":
   - Extract prefix, pathID from NLRI via `shared.ParseNLRIValue()` (for routeKey)
   - Extract raw hex: AttrHex from `event.RawAttributes`, NLRIHex from split of `event.RawNLRI[family]` (simple families) or `prefixToWire()` fallback
   - Convert next-hop IP to wire hex: `net.ParseIP(op.NextHop).To4()` → hex encode
   - Assign monotonic sequence index
   - Store in `ribIn[sourcePeer][routeKey] = RawRoute{Family, AttrHex, NHopHex, NLRIHex, SeqIndex}`
4. handleReceived: for each FamilyOperation with action="del":
   - Remove from `ribIn[sourcePeer][routeKey]`
5. `"adj-rib-in replay"` command (via execute-command callback):
   - Args: target peer address, from-index
   - Iterate ribIn for ALL source peers except target
   - Filter routes with sequenceIndex >= from-index
   - For each RawRoute: build `"update hex attr set <AttrHex> nhop set <NHopHex> nlri <Family> add <NLRIHex>"` → `updateRoute(target, cmd)`
   - Return `{status: "done", data: "{\"last-index\": maxSequenceIndex}"}`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin | JSON events via DirectBridge ("received" direction, format=full) | [ ] |
| Plugin → Engine | updateRoute RPC (replay "update hex" commands to target peer) | [ ] |
| Plugin B → Engine → This Plugin | dispatch-command RPC → execute-command callback | [ ] |

### Integration Points
- `internal/plugin/registry/` — plugin registration (name, YANG, commands)
- `internal/plugin/all/all.go` — blank import for init()
- Engine event delivery — "update direction received" subscription
- Engine command dispatch — "adj-rib-in replay", "adj-rib-in show", "adj-rib-in status" via OnExecuteCommand
- dispatch-command RPC (spec rib-01) — enables other plugins to invoke replay

### Architectural Verification
- [x] No bypassed layers
- [x] No unintended coupling (independent plugin, communicates via events + commands)
- [x] No duplicated functionality (shared package eliminates duplication between plugins)
- [x] All families stored (no isSimplePrefixFamily filter)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin registers as "bgp-adj-rib-in" | Registry lookup succeeds |
| AC-2 | Received UPDATE (format=full) with IPv4 route | RawRoute stored in ribIn[sourcePeer] with AttrHex, NHopHex, NLRIHex from raw event fields |
| AC-3 | Received UPDATE with IPv6 VPN route | Route stored (no family filtering) |
| AC-4 | Received withdrawal | Route removed from ribIn[sourcePeer] |
| AC-5 | "adj-rib-in status" command | Returns route count per source peer |
| AC-6 | "adj-rib-in show \<peer\>" command | Returns routes from source peer in human-readable JSON (decoded from stored hex for display) |
| AC-7 | "adj-rib-in replay X 0" via dispatch-command | Replays ALL routes from all sources except X, returns last-index |
| AC-8 | "adj-rib-in replay X 500" via dispatch-command | Replays only routes with index >= 500 |
| AC-9 | Source peer goes down | ribIn[peer] cleared |
| AC-10 | Routes stored have monotonic sequence index | Index increases with each insert |
| AC-11 | bgp-rib still works after shared extraction | Existing bgp-rib tests pass unchanged |
| AC-12 | `make ze-verify` passes | All tests pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseEvent` | `internal/plugin/bgp/shared/event_test.go` | Event parsing works from shared package | |
| `TestFormatRouteCommand` | `internal/plugin/bgp/shared/format_test.go` | Route formatting works from shared package | |
| `TestStoreReceivedRoute` | `bgp-adj-rib-in/rib_test.go` | RawRoute stored with AttrHex, NHopHex, NLRIHex from format=full event | |
| `TestStoreAllFamilies` | `bgp-adj-rib-in/rib_test.go` | VPN, EVPN, FlowSpec routes stored (no filtering) | |
| `TestRemoveWithdrawnRoute` | `bgp-adj-rib-in/rib_test.go` | Withdrawal removes route from ribIn | |
| `TestReplayAllSources` | `bgp-adj-rib-in/rib_test.go` | Replay sends "update hex" commands from A,B to X, excludes X's own | |
| `TestReplayFromIndex` | `bgp-adj-rib-in/rib_test.go` | Replay from non-zero index sends only newer routes | |
| `TestReplayReturnsLastIndex` | `bgp-adj-rib-in/rib_test.go` | Response includes last-index value as JSON data | |
| `TestSequenceIndexMonotonic` | `bgp-adj-rib-in/rib_test.go` | Each insert gets increasing index | |
| `TestClearPeerOnDown` | `bgp-adj-rib-in/rib_test.go` | Peer down clears that peer's routes | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| from-index | 0 to uint64 max | uint64 max | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `adj-rib-in-store-replay` | `test/plugin/adj-rib-in-store-replay.ci` | Routes received, stored, replayed to another peer via dispatch-command | |

## Files to Create

### Shared types package
- `internal/plugin/bgp/shared/route.go` — Route struct (from bgp-rib rib.go:70-88)
- `internal/plugin/bgp/shared/event.go` — Event, FamilyOperation, MessageInfo, ParseEvent (from bgp-rib event.go)
- `internal/plugin/bgp/shared/format.go` — FormatRouteCommand (from bgp-rib rib_commands.go)
- `internal/plugin/bgp/shared/nlri.go` — ParseNLRIValue, RouteKey (from bgp-rib rib_nlri.go)
- `internal/plugin/bgp/shared/event_test.go` — tests for shared event parsing
- `internal/plugin/bgp/shared/format_test.go` — tests for shared formatting

### bgp-adj-rib-in plugin
- `internal/plugins/bgp-adj-rib-in/register.go` — registers as "bgp-adj-rib-in"
- `internal/plugins/bgp-adj-rib-in/rib.go` — AdjRIBInManager, RawRoute (AttrHex/NHopHex/NLRIHex/Family/SeqIndex), ribIn map, handleReceived (raw hex from format=full), sequence index
- `internal/plugins/bgp-adj-rib-in/rib_commands.go` — command handlers (status, show, replay via "update hex") via OnExecuteCommand
- `internal/plugins/bgp-adj-rib-in/schema/embed.go` — YANG schema embed
- `internal/plugins/bgp-adj-rib-in/schema/ze-adj-rib-in.yang` — YANG schema
- `internal/plugins/bgp-adj-rib-in/rib_test.go` — unit tests
- `test/plugin/adj-rib-in-store-replay.ci` — functional test

## Files to Modify

- `internal/plugins/bgp-rib/rib.go` — import Route from shared package, remove local definition
- `internal/plugins/bgp-rib/event.go` — replace with import from shared package (or thin wrapper)
- `internal/plugins/bgp-rib/rib_commands.go` — import formatRouteCommand from shared package
- `internal/plugins/bgp-rib/rib_nlri.go` — import parseNLRIValue, routeKey from shared; keep isSimplePrefixFamily locally
- `internal/plugin/all/all.go` — add blank import for bgp-adj-rib-in
- `internal/plugin/all/all_test.go` — update expected plugin count

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | `internal/plugins/bgp-adj-rib-in/schema/` |
| Plugin registration | [x] | register.go with init() |
| all.go blank import | [x] | `internal/plugin/all/all.go` |
| Command registration | [x] | "adj-rib-in status", "adj-rib-in show", "adj-rib-in replay" via OnExecuteCommand |
| Functional test | [x] | `test/plugin/adj-rib-in-store-replay.ci` |
| Documentation | [x] | Document `internal/plugin/` vs `internal/plugins/` distinction |

## Implementation Steps

1. **Extract shared types** — move Route, Event, FamilyOperation, formatRouteCommand, parseNLRIValue, routeKey to `internal/plugin/bgp/shared/`
2. **Update bgp-rib imports** — bgp-rib imports from shared package, verify all existing bgp-rib tests pass
3. **Write adj-rib-in unit tests** (TDD) → Verify FAIL
4. **Create bgp-adj-rib-in plugin** — AdjRIBInManager with RawRoute storage (AttrHex/NHopHex/NLRIHex), handleReceived extracts raw hex from format=full events
5. **Subscribe to "received" events** — "update direction received" + "state" (all families, no filtering), format=full
6. **Implement replay command** — "adj-rib-in replay" via OnExecuteCommand, builds `update hex attr set ... nhop set ... nlri FAM add ...` commands, returns `{status, data}` with last-index
7. **Register in all.go** — blank import
8. **Write functional test** — adj-rib-in-store-replay.ci (exercises dispatch-command path)
9. **Run `make ze-verify`** → paste output
10. **Document plugin vs plugins distinction** — update architecture docs
11. **Critical Review** — all 6 quality checks

### Failure Routing

| Failure | Route To |
|---------|----------|
| Shared package import cycle | Step 1 (check dependency direction: shared has no plugin imports) |
| bgp-rib tests break after extraction | Step 2 (field names or function signatures changed) |
| Event parsing wrong | Step 4 (compare with bgp-rib's original parseEvent) |
| Non-unicast families lost | Step 5 (verify no isSimplePrefixFamily filter) |
| Replay returns wrong routes | Step 6 (check source peer exclusion, index filtering) |
| Replay response not received | Check spec rib-01 (dispatch-command must work first) |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| prefixToWireHex works for all families | Only correct for simple prefix families (IPv4/IPv6 unicast/multicast). VPN/EVPN need RD+labels+prefix, not bare prefix bytes | Critical Review check 1 — traced code path for VPN routes | Bug: stored wrong NLRIHex for complex families, replay would fail |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| if-else chain for NLRI hex selection | gocritic ifElseChain lint rule requires switch for 3+ branches | Boolean switch with case/default |

## Design Insights

- bgp-adj-rib-in is a NEW plugin alongside bgp-rib, not a rename
- ~~Uses parsed Route structs (like ribOut)~~ Uses raw hex wire bytes from format=full events — avoids JSON→struct→text→parse round-trip
- Replay uses `update hex attr set ... nhop set ... nlri FAM add ...` — engine decodes hex directly to wire bytes
- Shared package (Route, Event, FormatRouteCommand) stays: used by bgp-rib's ribOut and adj-rib-in's show command for human-readable output
- Subscribes to "received" events (bgp-rib only subscribes to "sent")
- No family filtering — all families stored (VPN, EVPN, FlowSpec, etc.)
- Monotonic sequence index enables incremental replay (from-index)
- Shared types in `internal/plugin/bgp/shared/` eliminate duplication between bgp-rib, bgp-adj-rib-in, and future bgp plugins
- `internal/plugin/` = infrastructure to support plugins; `internal/plugins/` = plugin implementations
- bgp-rib is NOT modified functionally in this spec — only import paths change

## Implementation Summary

### What Was Implemented
- Shared BGP types package (`internal/plugin/bgp/shared/`) with Event, Route, FamilyOperation, FormatRouteCommand, ParseNLRIValue, RouteKey extracted from bgp-rib
- bgp-adj-rib-in plugin (`internal/plugins/bgp-adj-rib-in/`) with RawRoute storage, handleReceived (raw hex from format=full), handleState (peer up/down), replay via OnExecuteCommand, YANG schema
- bgp-rib imports updated to use shared package (type aliases preserve API)
- Plugin registered in all.go, count updated in all_test.go
- Complex family NLRI handling: raw blob used for VPN/EVPN/FlowSpec instead of prefixToWireHex

### Bugs Found/Fixed
- VPN/EVPN NLRI storage: prefixToWireHex produced bare IPv4 prefix bytes for complex families (missing RD+labels). Fixed by using raw NLRI blob from format=full events when family is not simple prefix format

### Documentation Updates
- `docs/architecture/system-architecture.md` — added bgp-adj-rib-in to built-in plugins table, clarified rib vs adj-rib-in responsibilities
- `docs/architecture/api/architecture.md` — added Adj-RIB-In plugin and shared BGP types to implementation status table

### Deviations from Plan
- Functional test `adj-rib-in-store-replay.ci` deferred to spec rib-03: requires dispatch-command triggering from another plugin (bgp-rr), and Python ze_api module lacks dispatch-command support. Created `plugin-adj-rib-in-features.ci` instead.
- Shared package tests (`event_test.go`, `format_test.go`) not in original TDD plan but added to cover extracted code independently of bgp-rib

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Extract shared types from bgp-rib | ✅ Done | `internal/plugin/bgp/shared/` | event.go, route.go, format.go, nlri.go |
| bgp-adj-rib-in plugin | ✅ Done | `internal/plugins/bgp-adj-rib-in/` | rib.go, rib_commands.go, register.go |
| Subscribe to received events format=full | ✅ Done | `rib.go:109` | `SetStartupSubscriptions(["update direction received", "state"], nil, "full")` |
| Store routes keyed by source peer | ✅ Done | `rib.go:70` | `ribIn map[string]map[string]*RawRoute` |
| Store raw hex (AttrHex, NHopHex, NLRIHex) | ✅ Done | `rib.go:55-61` | RawRoute struct |
| Replay via update hex commands | ✅ Done | `rib.go:283-285` | `formatHexCommand` builds update hex command |
| All families stored (no filter) | ✅ Done | `rib.go:164` | Iterates all FamilyOps without filtering |
| Monotonic sequence index | ✅ Done | `rib.go:211` | `r.seqCounter++` on each insert |
| Commands via OnExecuteCommand | ✅ Done | `rib.go:104-106` | status, show, replay |
| YANG schema | ✅ Done | `schema/ze-adj-rib-in.yang` | Embedded via go:embed |
| Register in all.go | ✅ Done | `all.go:10` | Blank import |
| bgp-rib unchanged functionally | ✅ Done | `event.go` | Type aliases preserve API |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `all_test.go`, `plugin-adj-rib-in-features.ci` | Registry lookup + functional test |
| AC-2 | ✅ Done | `TestStoreReceivedRoute` | Verifies AttrHex, NHopHex, NLRIHex, SeqIndex |
| AC-3 | ✅ Done | `TestStoreAllFamilies` | VPN route stored with raw blob NLRIHex |
| AC-4 | ✅ Done | `TestRemoveWithdrawnRoute` | Route removed after withdrawal |
| AC-5 | ✅ Done | `TestHandleCommand_Status` | Returns running + route counts |
| AC-6 | ⚠️ Partial | `TestHandleCommand_Show` | Returns hex fields; decoded JSON deferred |
| AC-7 | ✅ Done | `TestReplayAllSources` | Excludes target peer, sends update hex commands |
| AC-8 | ✅ Done | `TestReplayFromIndex` | Only routes with SeqIndex >= from-index |
| AC-9 | ✅ Done | `TestClearPeerOnDown` | ribIn cleared on state=down |
| AC-10 | ✅ Done | `TestSequenceIndexMonotonic` | Each insert gets increasing unique index |
| AC-11 | ✅ Done | `go test ./internal/plugins/bgp-rib/...` | All bgp-rib tests pass |
| AC-12 | ✅ Done | `make ze-verify` | All 6 suites pass |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestParseEvent | ✅ Done | `shared/event_test.go` | 7 tests covering ze-bgp format, state, raw, peers, multi-family, invalid |
| TestFormatRouteCommand | ✅ Done | `shared/format_test.go` | 5 tests covering minimal, full, path-id, IPv6, ext communities |
| TestStoreReceivedRoute | ✅ Done | `bgp-adj-rib-in/rib_test.go` | AC-2 |
| TestStoreAllFamilies | ✅ Done | `bgp-adj-rib-in/rib_test.go` | AC-3 + VPN raw blob regression |
| TestRemoveWithdrawnRoute | ✅ Done | `bgp-adj-rib-in/rib_test.go` | AC-4 |
| TestReplayAllSources | ✅ Done | `bgp-adj-rib-in/rib_test.go` | AC-7 |
| TestReplayFromIndex | ✅ Done | `bgp-adj-rib-in/rib_test.go` | AC-8 |
| TestReplayReturnsLastIndex | ✅ Done | `bgp-adj-rib-in/rib_test.go` | AC-7 last-index |
| TestSequenceIndexMonotonic | ✅ Done | `bgp-adj-rib-in/rib_test.go` | AC-10 |
| TestClearPeerOnDown | ✅ Done | `bgp-adj-rib-in/rib_test.go` | AC-9 |
| adj-rib-in-store-replay | 🔄 Changed | `plugin-adj-rib-in-features.ci` | Deferred to rib-03 (needs dispatch-command trigger) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/bgp/shared/route.go` | ✅ Created | Route struct, RouteKey |
| `internal/plugin/bgp/shared/event.go` | ✅ Created | Event, ParseEvent, FamilyOperation, MessageInfo, peer helpers |
| `internal/plugin/bgp/shared/format.go` | ✅ Created | FormatRouteCommand |
| `internal/plugin/bgp/shared/nlri.go` | ✅ Created | ParseNLRIValue |
| `internal/plugin/bgp/shared/event_test.go` | ✅ Created | 7 tests |
| `internal/plugin/bgp/shared/format_test.go` | ✅ Created | 5 tests |
| `internal/plugins/bgp-adj-rib-in/register.go` | ✅ Created | init() registration |
| `internal/plugins/bgp-adj-rib-in/rib.go` | ✅ Created | AdjRIBInManager, RawRoute, handlers |
| `internal/plugins/bgp-adj-rib-in/rib_commands.go` | ✅ Created | status, show, replay handlers |
| `internal/plugins/bgp-adj-rib-in/schema/embed.go` | ✅ Created | go:embed |
| `internal/plugins/bgp-adj-rib-in/schema/ze-adj-rib-in.yang` | ✅ Created | YANG schema |
| `internal/plugins/bgp-adj-rib-in/rib_test.go` | ✅ Created | 15 unit tests |
| `test/plugin/adj-rib-in-store-replay.ci` | 🔄 Changed | Deferred; created features.ci instead |
| `test/plugin/plugin-adj-rib-in-features.ci` | ✅ Created | Plugin registration functional test |
| `internal/plugins/bgp-rib/event.go` | ✅ Modified | Type aliases to shared package |
| `internal/plugins/bgp-rib/rib.go` | ✅ Modified | Removed local Route definition |
| `internal/plugins/bgp-rib/rib_commands.go` | ✅ Modified | Removed local formatRouteCommand |
| `internal/plugins/bgp-rib/rib_nlri.go` | ✅ Modified | Removed local parseNLRIValue, routeKey |
| `internal/plugin/all/all.go` | ✅ Modified | Added blank import |
| `internal/plugin/all/all_test.go` | ✅ Modified | Updated expected plugin list |
| `cmd/ze/main_test.go` | ✅ Modified | Updated expected count |
| `docs/architecture/system-architecture.md` | ✅ Modified | Added adj-rib-in to plugin table |
| `docs/architecture/api/architecture.md` | ✅ Modified | Added to implementation status |

### Audit Summary
- **Total items:** 45
- **Done:** 43
- **Partial:** 1 (AC-6: show returns hex; decoded JSON deferred)
- **Skipped:** 0
- **Changed:** 1 (functional test deferred to rib-03)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (plugin vs plugins distinction)
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
