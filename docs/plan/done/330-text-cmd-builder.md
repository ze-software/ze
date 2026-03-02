# Spec: text-cmd-builder

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/bgp/shared/format.go` — current FormatRouteCommand
4. `internal/plugin/bgp/shared/route.go` — shared Route struct
5. `internal/plugins/bgp-watchdog/command.go` — current cmdBuilder

## Task

Consolidate the two duplicate text command builders (`shared.FormatRouteCommand` and `bgp-watchdog.cmdBuilder`) into a single shared implementation in `internal/plugin/bgp/shared/`. Both build `update text` command strings from route attributes — the duplication violates single responsibility and will diverge over time.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` — text command syntax
  → Constraint: Text parser accepts both long-form and short-form keywords via ResolveAlias
  → Decision: Consolidated builder should use canonical (long-form) keywords for clarity
- [ ] `docs/architecture/rib-transition.md` — engine stateless target
  → Constraint: Plugins inject routes via `update text` commands; shared builder serves all plugins

### Source Files Read
- [ ] `internal/plugin/bgp/shared/format.go` (80L) — FormatRouteCommand: announce only, long-form keywords, no VPN
  → Constraint: Used by bgp-rib plugin (handleReplayRequest, handleReplayAllPeers)
  → Constraint: Uses `*shared.Route` struct as input
- [ ] `internal/plugin/bgp/shared/route.go` (46L) — Route struct: missing RD and Labels fields
  → Constraint: Route has Family, Prefix, PathID, NextHop, Origin, ASPath, MED, LocalPreference, Communities, LargeCommunities, ExtendedCommunities
  → Constraint: MED and LocalPreference are `*uint32` (nil = omit)
- [ ] `internal/plugin/bgp/shared/format_test.go` — 5 unit tests for FormatRouteCommand
  → Constraint: Tests verify exact output strings; must preserve
- [ ] `internal/plugins/bgp-watchdog/command.go` (126L) — cmdBuilder: announce + withdraw, short-form keywords, with VPN (RD, labels)
  → Constraint: Used by bgp-watchdog config parser (buildCmdFromAttrs)
  → Constraint: Uses short-form keywords: pref, path, s-com, l-com, e-com, info
  → Constraint: Has routeKey() method
- [ ] `internal/plugins/bgp-watchdog/command_test.go` — 3 test functions (19 subtests) for cmdBuilder
  → Constraint: Tests verify exact output strings including VPN fields
- [ ] `internal/plugins/bgp-watchdog/config.go` — buildCmdFromAttrs creates cmdBuilder, calls announce()/withdraw()/routeKey()
  → Constraint: Config parser populates cmdBuilder fields from JSON tree
- [ ] `internal/plugins/bgp-rib/rib.go:420` — calls FormatRouteCommand in handleReplayRequest
  → Constraint: Builds announce command from stored Route for replay
- [ ] `internal/plugins/bgp-rib/rib_commands.go:193` — calls FormatRouteCommand in handleReplayAllPeers
  → Constraint: Same pattern as above, iterating all peers
- [ ] `internal/plugins/bgp/handler/update_text.go:456` — ResolveAlias maps short→long keywords
  → Decision: Both keyword styles are accepted; using long-form in builder is canonical

**Key insights:**
- Two builders do the same thing (route attrs → "update text" string) with different scopes
- `cmdBuilder` is a strict superset: has withdraw, VPN fields, routeKey
- Parser accepts both keyword styles; canonical long-form is clearer and self-documenting
- `shared.Route` needs RD and Labels fields to support VPN routes
- bgp-rib caller uses `*Route` directly; watchdog caller would need to build a `Route` from config

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/bgp/shared/format.go` — FormatRouteCommand(*Route) string
- [ ] `internal/plugin/bgp/shared/route.go` — Route struct, RouteKey() function
- [ ] `internal/plugins/bgp-watchdog/command.go` — cmdBuilder struct, announce(), withdraw(), routeKey()
- [ ] `internal/plugins/bgp-watchdog/config.go` — buildCmdFromAttrs() creates cmdBuilder

**Behavior to preserve:**
- Exact wire bytes from all functional tests (encode, plugin, exabgp watchdog)
- bgp-rib replay produces valid `update text` commands
- bgp-watchdog config produces valid announce and withdraw commands
- VPN routes with RD and labels work correctly

**Behavior to change:**
- `cmdBuilder` deleted from bgp-watchdog, replaced by shared functions
- `FormatRouteCommand` extended with withdraw support and VPN fields
- Keyword style unified to long-form (canonical)

## Data Flow (MANDATORY)

### Entry Points

| Entry | Current Path | Target Path |
|-------|-------------|-------------|
| bgp-rib replay | Route → FormatRouteCommand → string | Route → FormatAnnounceCommand → string |
| watchdog announce | config → cmdBuilder → announce() → string | config → Route → FormatAnnounceCommand → string |
| watchdog withdraw | config → cmdBuilder → withdraw() → string | config → Route → FormatWithdrawCommand → string |

### Transformation Path
1. Caller builds a `shared.Route` (extended with RD, Labels)
2. `FormatAnnounceCommand(route)` builds "update text [attrs] nlri [family] add [prefix]"
3. `FormatWithdrawCommand(route)` builds "update text nlri [family] del [prefix]"
4. Engine text parser processes the string (ResolveAlias normalizes keywords)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → Engine | "update text ..." string via UpdateRoute | [ ] |

### Integration Points
- `shared.FormatRouteCommand` callers in bgp-rib (rename to FormatAnnounceCommand)
- `cmdBuilder` callers in bgp-watchdog/config.go (switch to shared.Route + Format functions)
- `shared.Route` struct (add RD, Labels fields)

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (shared package remains leaf — no plugin imports)
- [ ] No duplicated functionality (eliminates duplication)
- [ ] Zero-copy preserved (strings.Builder pattern unchanged)

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| bgp-rib route replay with VPN route | → | FormatAnnounceCommand with RD/labels | `TestFormatAnnounceCommand/vpn_with_rd_and_label` |
| bgp-watchdog config with watchdog routes | → | FormatAnnounceCommand + FormatWithdrawCommand | `test/plugin/watchdog.ci` |
| bgp-watchdog VPN watchdog route | → | FormatAnnounceCommand with RD/labels | `TestFormatAnnounceCommand/vpn_with_rd_and_label` |
| ExaBGP compat watchdog test | → | FormatAnnounceCommand + FormatWithdrawCommand | `test/exabgp-compat` test `a` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route with standard attributes | FormatAnnounceCommand produces valid `update text` with long-form keywords |
| AC-2 | Route with RD and labels | FormatAnnounceCommand includes `rd` and `label` modifiers |
| AC-3 | Route for withdrawal | FormatWithdrawCommand produces `update text nlri [family] del [prefix]` |
| AC-4 | Route with path-id | Both commands include `path-information N` modifier |
| AC-5 | bgp-rib replay uses new function | Existing replay tests pass unchanged |
| AC-6 | bgp-watchdog uses shared builder | Functional tests (plugin + exabgp watchdog) pass |
| AC-7 | cmdBuilder deleted from bgp-watchdog | No duplicate builder exists |
| AC-8 | shared.Route extended with RD/Labels | Fields present and used by formatter |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFormatAnnounceCommand` (existing, extended) | `shared/format_test.go` | Standard route → announce string | |
| `TestFormatAnnounceCommand/vpn_with_rd_and_label` | `shared/format_test.go` | VPN route with RD + labels | |
| `TestFormatAnnounceCommand/nhop_self` | `shared/format_test.go` | nhop self keyword | |
| `TestFormatAnnounceCommand/all_attributes` | `shared/format_test.go` | All attributes combined | |
| `TestFormatWithdrawCommand` | `shared/format_test.go` | Route → withdraw string | |
| `TestFormatWithdrawCommand/with_path_id` | `shared/format_test.go` | Withdraw with path-information | |
| `TestFormatWithdrawCommand/vpn_with_rd_and_label` | `shared/format_test.go` | VPN withdrawal with RD + labels | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| watchdog.ci | `test/plugin/watchdog.ci` | Watchdog announce/withdraw cycle | |
| exabgp conf-watchdog | `test/exabgp-compat/` | ExaBGP watchdog compat | |

## Files to Modify

- `internal/plugin/bgp/shared/route.go` — Add RD (string) and Labels ([]uint32) fields to Route
- `internal/plugin/bgp/shared/format.go` — Rename FormatRouteCommand → FormatAnnounceCommand, add FormatWithdrawCommand, add VPN modifier support (RD, labels)
- `internal/plugin/bgp/shared/format_test.go` — Extend tests for VPN, withdrawal, rename
- `internal/plugins/bgp-rib/rib.go` — Update FormatRouteCommand → FormatAnnounceCommand call
- `internal/plugins/bgp-rib/rib_commands.go` — Update FormatRouteCommand → FormatAnnounceCommand call
- `internal/plugins/bgp-rib/rib_test.go` — Update FormatRouteCommand → FormatAnnounceCommand calls
- `internal/plugins/bgp-watchdog/config.go` — Replace cmdBuilder usage with shared.Route + Format functions
- `internal/plugins/bgp-watchdog/command.go` — Delete file (cmdBuilder eliminated)
- `internal/plugins/bgp-watchdog/command_test.go` — Delete file (tests moved to shared/format_test.go)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | No new RPCs |
| CLI commands/flags | No | No CLI changes |
| Plugin registry | No | No new plugins |
| Functional test | Yes | Existing tests verify — no new .ci files needed |
| Architecture docs | No | Internal refactoring only |

## Files to Create

None — this is a consolidation refactoring, not new functionality.

## Implementation Steps

### Step 1: Extend shared.Route and format functions
1. Add `RD string` and `Labels []uint32` to Route struct
2. Rename FormatRouteCommand → FormatAnnounceCommand
3. Add VPN modifier support (RD, labels) to FormatAnnounceCommand
4. Add FormatWithdrawCommand function
5. Switch from short-form to long-form keywords where they differ
6. Write new tests → verify FAIL → implement → verify PASS
7. Review: all cmdBuilder subtests ported? VPN fields covered?

### Step 2: Update bgp-rib callers
1. Update rib.go and rib_commands.go: FormatRouteCommand → FormatAnnounceCommand
2. Update rib_test.go references
3. Review: tests still pass? No behavioral change for rib?

### Step 3: Update bgp-watchdog to use shared builder
1. Rewrite config.go buildCmdFromAttrs to build shared.Route instead of cmdBuilder
2. Call FormatAnnounceCommand/FormatWithdrawCommand instead of announce()/withdraw()
3. Use shared.RouteKey instead of cmdBuilder.routeKey (or extend RouteKey for RD)
4. Review: all config attributes mapped to Route fields?

### Step 4: Delete cmdBuilder
1. Delete command.go
2. Delete command_test.go
3. Update Related: comments in remaining bgp-watchdog files
4. Review: no orphaned references?

### Step 5: Verify
1. Run `make test-all`
2. Verify functional tests pass (plugin watchdog, exabgp watchdog)
3. Critical Review

### Failure Routing
| Failure | Route To |
|---------|----------|
| Keyword change breaks parser | Check ResolveAlias handles both forms; if not, keep short-form |
| bgp-rib test expects exact string | Update test to match new long-form keywords |
| watchdog functional test fails | Compare wire bytes — keyword change shouldn't affect wire |
| RouteKey format incompatible | Extend shared.RouteKey for RD support |

## Mistake Log

No mistakes — straightforward consolidation.

## Design Insights

- Text parser's ResolveAlias means keyword choice (long vs short) has zero wire-level impact
- `cmdBuilder.routeKey()` uses a different format than `shared.RouteKey()` — kept as standalone `watchdogRouteKey` since the format is internal to the pool
- shared package remains a leaf (no plugin imports) — safe for cross-plugin use

## Implementation Summary

Consolidated two duplicate text command builders into `internal/plugin/bgp/shared/`:

| File | Change | Why |
|------|--------|-----|
| `shared/route.go` | Added RD, Labels fields | VPN route support for shared builder |
| `shared/format.go` | Added FormatAnnounceCommand, FormatWithdrawCommand, writeNLRIModifiers; deprecated FormatRouteCommand | Single implementation for all callers |
| `shared/format_test.go` | Added 3 test functions (VPN, nhop self, withdrawals) | Cover new functionality |
| `bgp-rib/event.go` | Updated alias to FormatAnnounceCommand | Use new canonical name |
| `bgp-watchdog/config.go` | Rewrote buildCmdFromAttrs → buildRouteFromAttrs, parseNLRIEntries uses shared.Route | Eliminate cmdBuilder dependency |
| `bgp-watchdog/config_test.go` | Updated 2 test expectations to long-form keywords | Match new canonical output |
| `bgp-watchdog/server.go` | Updated Related: comments | Removed stale command.go reference |
| `bgp-watchdog/command.go` | Deleted | cmdBuilder eliminated |
| `bgp-watchdog/command_test.go` | Deleted | Tests covered by shared/format_test.go |

## Deviations

| Spec Item | Deviation | Reason |
|-----------|-----------|--------|
| Step 2: update rib.go, rib_commands.go | Changed alias in event.go only | bgp-rib uses package-level alias `formatRouteCommand = shared.FormatAnnounceCommand`; callers unchanged |
| Step 3: use shared.RouteKey | Used standalone watchdogRouteKey | Pool key format (`prefix#pathID`) differs from shared format (`family:prefix[:pathID]`); internal only |
| FormatRouteCommand renamed | Kept as deprecated alias | Cleaner than breaking rename; existing tests validate alias path |

## Implementation Audit

| AC ID | Status | Demonstrated By |
|-------|--------|-----------------|
| AC-1 | ✅ Done | TestFormatRouteCommand_MinimalRoute, TestFormatRouteCommand_FullAttributes |
| AC-2 | ✅ Done | TestFormatAnnounceCommand_VPN (shared/format_test.go) |
| AC-3 | ✅ Done | TestFormatWithdrawCommand (4 subtests: basic, ipv6, path-id, vpn) |
| AC-4 | ✅ Done | TestFormatRouteCommand_WithPathID, TestFormatWithdrawCommand/with_path-id |
| AC-5 | ✅ Done | bgp-rib tests all pass; alias updated in event.go:23 |
| AC-6 | ✅ Done | Functional 263/263, ExaBGP 37/37 |
| AC-7 | ✅ Done | command.go and command_test.go deleted |
| AC-8 | ✅ Done | route.go:38-39 (RD, Labels); writeNLRIModifiers uses them |

| TDD Test | Status |
|----------|--------|
| TestFormatAnnounceCommand_VPN | ✅ Pass |
| TestFormatAnnounceCommand_NhopSelf | ✅ Pass |
| TestFormatWithdrawCommand (4 subtests) | ✅ Pass |
| Existing FormatRouteCommand tests (5) | ✅ Pass |
| TestParseConfigBasic (updated expect) | ✅ Pass |
| TestParseConfigAllAttributes (updated expect) | ✅ Pass |

| File in Plan | Status |
|-------------|--------|
| shared/route.go | ✅ Modified |
| shared/format.go | ✅ Modified |
| shared/format_test.go | ✅ Modified |
| bgp-rib/event.go | ✅ Modified (alias only) |
| bgp-rib/rib.go | 🔄 Changed — alias in event.go handles this |
| bgp-rib/rib_commands.go | 🔄 Changed — alias in event.go handles this |
| bgp-rib/rib_test.go | 🔄 Changed — no change needed, uses alias |
| bgp-watchdog/config.go | ✅ Modified |
| bgp-watchdog/command.go | ✅ Deleted |
| bgp-watchdog/command_test.go | ✅ Deleted |

## Critical Review

| Check | Result |
|-------|--------|
| Correctness | Pass — all unit, functional, and exabgp tests pass |
| Simplicity | Pass — eliminated duplicate builder, single shared implementation |
| Modularity | Pass — shared package remains leaf; config.go ~300L |
| Consistency | Pass — uses same patterns as existing shared package |
| Completeness | Pass — no TODOs, no deferred items |
| Quality | Pass — lint 0 issues; no debug statements |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
