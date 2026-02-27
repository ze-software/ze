# Spec: Peer Text Search/Filter

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research and constraints
4. `cmd/ze-chaos/web/handlers.go` - handlePeers with sort/status filter logic
5. `cmd/ze-chaos/web/render.go` - filter bar in writeLayout
6. `cmd/ze-chaos/web/assets/style.css` - filter bar styles

## Task

Add a text search/filter input to the peer table filter bar. The input filters peers by index, address, ASN, or status text. The filter is applied server-side: HTMX sends a GET /peers request with a search query param. Input is debounced via HTMX hx-trigger="keyup changed delay:200ms" to avoid excessive requests. The search filter combines with the existing status filter using AND logic -- both must match for a peer to appear. handlePeers in handlers.go already supports sort and status filter, so this adds a search parameter following the same pattern. No new RPCs or SSE events are needed.

**Parent spec:** `docs/plan/spec-chaos-ux-0-umbrella.md`

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research, constraints, source file survey
  → Constraint: All rendering is server-side Go HTML; no client-side JS filtering
  → Constraint: HTMX + SSE architecture, no JS framework
  → Decision: Dark theme with CSS custom properties
- [ ] `docs/architecture/chaos-web-dashboard.md` - dashboard architecture
  → Constraint: handlePeers already supports sort by column + status filter including "fault" mode
  → Decision: Filter uses ps.Status.String() == statusFilter for string-based matching
  → Constraint: Active set manages visible peers (max 40) with priority-based promotion

**Key insights:**
- handlePeers in handlers.go accepts sort, dir, and status query params already
- Adding a search param follows the same pattern: read from r.URL.Query(), apply filter in the loop
- The filter bar is rendered in writeLayout in render.go with a status select dropdown
- PeerState has Index (int), address is derived from index (127.0.0.X pattern in chaos), Status (enum with String())
- The existing status filter iterates d.state.Active.Indices() and checks each peer
- HTMX debounce (delay:200ms) is a standard hx-trigger modifier, no JS needed
- hx-include attribute ensures sort/dir/status params are included with the search request
- The search input needs name="search" and hx-get="/peers" with hx-target="#peer-tbody"

## Current Behavior (MANDATORY)

**Source files read:** (see umbrella for full survey)
- [ ] `cmd/ze-chaos/web/handlers.go` (330L) - handlePeers reads sort/dir/status from query, gets active set indices, filters by status ("fault" special mode or string match), sorts, renders table body with writePeerRows
  → Constraint: handlePeers iterates d.state.Active.Indices() then filters; only active set peers shown
  → Decision: Status filter: "fault" shows Down/Reconnecting/Idle; other values match ps.Status.String()
  → Decision: Response wraps rows in tbody with id="peer-tbody"
- [ ] `cmd/ze-chaos/web/render.go` (470L) - writeLayout renders filter bar div with class="filters" containing a label and a status select dropdown; the select has hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML" and hx-include="[name='sort'],[name='dir']"
  → Constraint: writeLayout is single entry point for full page render
  → Decision: Filter bar is a flex row with gap 8px, margin-bottom 8px
- [ ] `cmd/ze-chaos/web/state.go` (594L) - PeerState has Index int, Status PeerStatus (enum), RoutesSent, RoutesRecv, ChaosCount, etc.; PeerStatus.String() returns "up"/"down"/"reconnecting"/"idle"/"syncing"
  → Constraint: Peer index is 0-based integer matching position in simulation
  → Decision: PeerStatus enum: Idle=0, Up=1, Down=2, Reconnecting=3, Syncing=4
- [ ] `cmd/ze-chaos/web/assets/style.css` (1066L) - .filters has display:flex gap:8px align-items:center; .filters select and .filters input already styled with bg-tertiary border-border color-text-primary padding-4px-8px border-radius-4px font-12px font-mono
  → Constraint: .filters input style already exists -- text input will inherit it automatically

**Behavior to preserve:**
- Existing status filter dropdown and its behavior unchanged
- Sort column and direction functionality unchanged
- Active set visibility and promotion logic unchanged
- HTMX swap target #peer-tbody for peer table updates
- Table row rendering by writePeerRows unchanged

**Behavior to change:**
- Add text input field to filter bar for peer search
- handlePeers gains a "search" query param
- Search text is matched against peer index (number), status text, and peer address (if available)
- Search filter combines with status filter using AND logic (both must match)
- Input is debounced with hx-trigger="keyup changed delay:200ms"
- Input includes existing sort/dir/status params via hx-include

## Data Flow (MANDATORY - see rules/data-flow-tracing.md)

### Entry Point
- User types in the search text input in the filter bar
- After 200ms debounce, HTMX sends GET /peers?search=<text>&status=<current>&sort=<col>&dir=<dir>

### Transformation Path
1. HTMX input fires after 200ms debounce on keyup (hx-trigger="keyup changed delay:200ms")
2. GET /peers with search param (plus existing sort/dir/status) reaches handlePeers
3. handlePeers reads search param from query string
4. handlePeers gets active set indices, applies status filter (existing), then applies search filter
5. Search filter: for each remaining peer, check if search text matches index (as string prefix), status text (contains), or address (contains)
6. Matching peers sorted and rendered as table rows via writePeerRows
7. Response returned as tbody HTML fragment

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser to Go | HTMX GET /peers?search=42 with 200ms debounced keyup trigger | [ ] |
| Go to Browser | HTML fragment with filtered peer table body (tbody id="peer-tbody") | [ ] |
| Filter to Active Set | Iterates Active.Indices(), applies status then search filters | [ ] |

### Integration Points
- `handlePeers()` in handlers.go - add search param reading and filter logic after status filter
- `writeLayout()` in render.go - add text input element in filter bar div with HTMX attributes
- Existing hx-include on status select - update to include [name='search'] so status change preserves search text

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| GET /peers?search=42 | → | handlePeers filters to peer 42 only | TestHandlePeersSearchSingle |
| GET /peers?search=down | → | handlePeers filters to peers with status containing "down" | TestHandlePeersSearchStatus |
| GET /peers?search=42&status=up | → | handlePeers applies both filters with AND logic | TestHandlePeersSearchWithStatusFilter |
| GET / full page | → | writeLayout includes search input in filter bar | TestLayoutIncludesSearchInput |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Full page load with filter bar visible | Text input is visible in the filter bar alongside status dropdown, with placeholder text |
| AC-2 | Type "42" in search input | After 200ms debounce, peer table shows only peer 42 (if in active set) |
| AC-3 | Type "down" in search input | Peer table shows only peers whose status text contains "down" |
| AC-4 | Type "42" with status filter "up" active | Only peer 42 shown AND only if its status is "up" (AND logic) |
| AC-5 | Clear search input (empty string) | All peers matching current status filter are shown (no search filter applied) |
| AC-6 | Type text with no matches | Empty peer table body (no rows, just empty tbody) |
| AC-7 | Type in search input then change status dropdown | Both filters applied together; search text preserved in input |
| AC-8 | Search input has 200ms debounce | No request fires until 200ms after last keyup (hx-trigger="keyup changed delay:200ms") |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerMatchesSearch_Index` | `cmd/ze-chaos/web/handlers_test.go` | peerMatchesSearch returns true when search text matches peer index as string | |
| `TestPeerMatchesSearch_Status` | `cmd/ze-chaos/web/handlers_test.go` | peerMatchesSearch returns true when search text is substring of status string | |
| `TestPeerMatchesSearch_Empty` | `cmd/ze-chaos/web/handlers_test.go` | peerMatchesSearch returns true for empty search (no filter) | |
| `TestPeerMatchesSearch_NoMatch` | `cmd/ze-chaos/web/handlers_test.go` | peerMatchesSearch returns false when search text matches nothing | |
| `TestPeerMatchesSearch_CaseInsensitive` | `cmd/ze-chaos/web/handlers_test.go` | peerMatchesSearch is case-insensitive (e.g., "UP" matches status "up") | |
| `TestHandlePeersSearchSingle` | `cmd/ze-chaos/web/handlers_test.go` | GET /peers?search=42 returns only peer 42 in response | |
| `TestHandlePeersSearchStatus` | `cmd/ze-chaos/web/handlers_test.go` | GET /peers?search=down returns only peers with "down" in status | |
| `TestHandlePeersSearchWithStatusFilter` | `cmd/ze-chaos/web/handlers_test.go` | GET /peers?search=42&status=up applies AND logic | |
| `TestHandlePeersSearchEmpty` | `cmd/ze-chaos/web/handlers_test.go` | GET /peers?search= returns all peers (no search filter) | |
| `TestHandlePeersSearchNoMatch` | `cmd/ze-chaos/web/handlers_test.go` | GET /peers?search=99999 returns empty tbody | |
| `TestLayoutIncludesSearchInput` | `cmd/ze-chaos/web/render_test.go` | writeLayout output includes input with name="search" and debounce trigger | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Peer index in search | 0 to PeerCount-1 | PeerCount-1 | N/A (negative numbers just don't match) | PeerCount (no match, empty result) |
| Search string length | 0-1000 | 1000 (reasonable max) | N/A (0 = no filter) | Very long string (no match, harmless) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-peer-filter` | `test/chaos/peer-filter.ci` | Load dashboard, type peer index in search input, verify filtered result | |

### Future (if deferring any tests)
- Regex-based search: not in scope, deferred to potential future spec
- ASN-based search: requires ASN field on PeerState (not currently available in chaos simulation)

## Files to Modify
- `cmd/ze-chaos/web/handlers.go` - add search param reading in handlePeers (r.URL.Query().Get("search")); add peerMatchesSearch(ps *PeerState, search string) bool helper; apply search filter after status filter in the peer loop
- `cmd/ze-chaos/web/render.go` - add text input element in filter bar div in writeLayout: input with name="search", type="text", placeholder="Search peers...", hx-get="/peers", hx-target="#peer-tbody", hx-swap="outerHTML", hx-trigger="keyup changed delay:200ms", hx-include="[name='sort'],[name='dir'],[name='status']"; also update status select hx-include to add [name='search']
- `cmd/ze-chaos/web/assets/style.css` - minimal or no changes needed (existing .filters input styles apply automatically)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

## Files to Create
- `test/chaos/peer-filter.ci` - functional test for peer search/filter

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for peerMatchesSearch** - Review: covers index match, status match, empty search, no match, case insensitivity?
2. **Run tests** - Verify FAIL (paste output). Fail for RIGHT reason (peerMatchesSearch does not exist)?
3. **Implement peerMatchesSearch in handlers.go** - Takes *PeerState and search string. Returns true if search is empty. Otherwise checks: index as string starts with search, or status string contains search (case-insensitive). Uses strings.Contains with strings.ToLower for case insensitivity.
4. **Run tests** - Verify PASS (paste output). All pass? Any flaky?
5. **Write integration tests for handlePeers with search param** - Review: single match, status text match, AND with status filter, empty search, no match?
6. **Run tests** - Verify FAIL.
7. **Add search param to handlePeers** - Read search from query. After status filter loop, add search filter: if search != "", filter indices to those where peerMatchesSearch returns true.
8. **Run tests** - Verify PASS.
9. **Write unit test for search input in layout** - Review: input has correct name, trigger, include attributes?
10. **Run tests** - Verify FAIL.
11. **Add text input to filter bar in writeLayout in render.go** - Input element with: name="search", type="text", placeholder="Search peers...", hx-get="/peers", hx-target="#peer-tbody", hx-swap="outerHTML", hx-trigger="keyup changed delay:200ms", hx-include="[name='sort'],[name='dir'],[name='status']". Also update status select hx-include to add [name='search'] so search is preserved when status changes.
12. **Run tests** - Verify PASS.
13. **Write functional test** - Create test/chaos/peer-filter.ci.
14. **Verify all** - make ze-lint and make ze-chaos-test.
15. **Critical Review** - All 6 checks from rules/quality.md must pass.
16. **Complete spec** - Fill audit tables, move to done/.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 7 (fix syntax/types) |
| Test fails wrong reason | Step 1 or 5 (fix test expectations) |
| Test fails behavior mismatch | Re-read handlePeers in handlers.go for actual filter loop structure |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong, revisit design; if AC correct, fix filter logic |
| Audit finds missing AC | Back to implementation for that criterion |
| Search breaks existing status filter | Verify hx-include on both input and select include each other's name |

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

## Implementation Summary

### What Was Implemented
- Added `peerMatchesSearch()` helper in handlers.go — matches peer index (string prefix) or status text (case-insensitive substring)
- Added `search` query param to `handlePeers` — applied after status filter with AND logic
- Added text input to filter bar in render.go — name="search", 200ms debounce, includes sort/dir/status params
- Updated status select hx-include to also send search param

### Bugs Found/Fixed
- None

### Documentation Updates
- None needed (no architecture docs affected)

### Deviations from Plan
- No `.ci` functional test — no chaos functional test infrastructure exists
- Search does not match peer address — chaos simulation peers don't have meaningful addresses exposed in PeerState

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Text search input in filter bar | ✅ Done | render.go:164 | Input with name="search" and placeholder |
| Server-side search filter in handlePeers | ✅ Done | handlers.go:143 | search query param applied after status filter |
| AND logic with status filter | ✅ Done | handlers.go:143-150 | Status filter first, then search filter |
| 200ms debounce | ✅ Done | render.go:166 | hx-trigger="keyup changed delay:200ms" |
| hx-include for combined params | ✅ Done | render.go:164,167 | Both input and select include each other's name |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | TestLayoutIncludesSearchInput | Input visible in filter bar |
| AC-2 | ✅ Done | TestHandlePeersSearchSingle | Search "2" returns peer 2 only |
| AC-3 | ✅ Done | TestHandlePeersSearchStatus | Search "down" returns down peers |
| AC-4 | ✅ Done | TestHandlePeersSearchWithStatusFilter | AND logic verified |
| AC-5 | ✅ Done | TestPeerMatchesSearch/empty_matches_all | Empty search returns all |
| AC-6 | ✅ Done | TestHandlePeersSearchNoMatch | No matches = empty tbody |
| AC-7 | ✅ Done | render.go hx-include | Status select includes [name='search'] |
| AC-8 | ✅ Done | TestLayoutIncludesSearchInput | delay:200ms in trigger |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestPeerMatchesSearch_Index | ✅ Done | handlers_test.go (table-driven) | index_exact + index_prefix cases |
| TestPeerMatchesSearch_Status | ✅ Done | handlers_test.go (table-driven) | status_contains + status_partial |
| TestPeerMatchesSearch_Empty | ✅ Done | handlers_test.go (table-driven) | empty_matches_all |
| TestPeerMatchesSearch_NoMatch | ✅ Done | handlers_test.go (table-driven) | no_match + index_no_match |
| TestPeerMatchesSearch_CaseInsensitive | ✅ Done | handlers_test.go (table-driven) | status_case_insensitive |
| TestHandlePeersSearchSingle | ✅ Done | handlers_test.go | Peer 2 only |
| TestHandlePeersSearchStatus | ✅ Done | handlers_test.go | Down peers only |
| TestHandlePeersSearchWithStatusFilter | ✅ Done | handlers_test.go | AND logic |
| TestHandlePeersSearchEmpty | 🔄 Changed | TestPeerMatchesSearch/empty | Covered by unit test |
| TestHandlePeersSearchNoMatch | ✅ Done | handlers_test.go | Empty tbody |
| TestLayoutIncludesSearchInput | ✅ Done | handlers_test.go | Input + debounce |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| cmd/ze-chaos/web/handlers.go | ✅ Modified | peerMatchesSearch + search param in handlePeers |
| cmd/ze-chaos/web/render.go | ✅ Modified | Search input in filter bar |
| cmd/ze-chaos/web/handlers_test.go | ✅ Modified | 14 new test cases |
| test/chaos/peer-filter.ci | ❌ Skipped | No chaos functional test infra |

### Audit Summary
- **Total items:** 25
- **Done:** 23
- **Partial:** 0
- **Skipped:** 1 (functional test)
- **Changed:** 1 (empty search covered by unit test instead of handler test)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`cmd/ze-chaos/web/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] `make ze-lint` passes
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** -- NEVER commit implementation without the completed spec. One commit = code + tests + spec.
