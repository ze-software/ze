# Spec: fixit-bgp-per-family-prefix-enforcement

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md` |
| Updated | 2026-07-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Three per-family YANG leaves are stored as single per-peer scalars. The last
family parsed wins. The operator's per-family intent is silently discarded.

The three leaves all live inside the per-family `prefix` container in
`internal/component/bgp/yang/ze-bgp-conf.yang`:

| Leaf | YANG line | YANG default | Stored as | Field line |
|------|-----------|--------------|-----------|------------|
| `teardown` | `ze-bgp-conf.yang` | `true` | `PrefixTeardown bool` | `peersettings.go` |
| `idle-timeout` | `ze-bgp-conf.yang` | `0` | `PrefixIdleTimeout uint16` | `peersettings.go` |
| `updated` | `ze-bgp-conf.yang` | none | `PrefixUpdated string` | `peersettings.go` |

The two sibling leaves in the same container are correctly per-family. They are
`PrefixMaximum map[string]uint32` and `PrefixWarning map[string]uint32`
(`peersettings.go` and `peersettings.go`).

All three scalar assignments happen in `parsePrefixLimitFromFamily`
(`internal/component/bgp/reactor/config.go`). The assignments are at
`config.go`, `config.go`, and `config.go`. The function is called
once per family from `config.go`.

### The mechanism, corrected

The defect is **deterministic**, not random. The enclosing loop sorts the family
keys, then iterates (`config.go`). The sort exists to keep
Multiprotocol capability ordering stable in the OPEN message. A comment at
`config.go` states that purpose.

The consequence is that the **last family in lexicographic key order wins**. For
a peer carrying `ipv4/unicast` and `ipv6/unicast`, the winner is always
`ipv6/unicast`, because the byte `4` sorts before the byte `6`.

The damage is wider than an explicit disagreement. YANG defaults are
materialized into each family list entry, and the parser then reads them.
`ApplyDefaults` recurses into every existing list entry
(`internal/component/config/schema_defaults.go`). The `prefix` container
is a non-presence container, so it is created and filled
(`schema_defaults.go`). A family that omits `teardown` therefore arrives
carrying the materialized value `true`.

That produces the worst case below.

1. The operator sets `teardown false` on `ipv4/unicast`.
2. The operator says nothing about `teardown` on `ipv6/unicast`.
3. `ApplyDefaults` gives `ipv6/unicast` the value `true`.
4. The sort puts `ipv6/unicast` last.
5. Both families become teardown-enabled.

A family that expressed **no opinion** overwrites a family that expressed an
explicit one. The operator asked for warn-only on IPv4 and got a session
teardown instead.

`updated` behaves slightly differently. It carries no YANG default, so it only
overwrites when a later family sets it explicitly.

### Blast radius in one line

Ze stops a session the operator asked to keep in warn-only mode. Or Ze keeps a
session the operator asked to stop. Both directions are reachable. The direction
depends only on which family sorts last.

### Provenance

This session found the defect during RFC 4486 requirement extraction for
`plan/spec-rfcgate-4-ledger.md`. That work covers the prefix-limit `Cease`
subcode path. That path reads `PrefixTeardown` to decide whether to send the
NOTIFICATION.

### Test coverage today

No test covers cross-family **enforcement**. Grep results are recorded in the
Assumptions table as A-3, A-4, and A-9. The three existing parser tests are all
single-family. They are `TestPrefixLimitConfigTeardownDefault`
(`config_test.go`), `TestPrefixLimitConfigTeardownFalse`
(`config_test.go`), and `TestPrefixLimitConfigIdleTimeout`
(`config_test.go`). Each declares only `ipv4/unicast`.

One test does exercise two families. `TestPrefixPerFamilyIsolation`
(`session_prefix_test.go`) gives `ipv6/unicast` its own maximum and warning,
then asserts that an IPv4 overflow leaves the IPv6 counter at zero. That covers
**counter and maximum** isolation only. Those are the two fields that are already
maps. It asserts nothing about `teardown`, `idle-timeout`, or `updated`. The
distinction matters, because the file already looks as though it has per-family
coverage.

### Why the defect stayed invisible

A single-family config can never expose it. There is no second family to
overwrite the first. Every parser test and every functional fixture that touches
these leaves is single-family.

The original diagnosis assumed random Go map iteration. That assumption was
wrong, and the correction matters for the test design. Because the loop sorts,
the wrong value is produced on **every** run, not on some runs. A test can
therefore assert the exact per-family outcome and stay deterministic. No
repeat-count loop and no retry are needed.

The test must assert **per-family state**. An aggregate assertion cannot see the
bug. Aggregate behavior looks self-consistent, because both families genuinely
share the one scalar.

### Goal

Honor all three leaves per family. Keep the sort. Remove the cross-family
overwrite. Correct the misleading comment at `config.go`, which claims
"Per-family prefix enforcement settings" directly above three scalar writes.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - how the config tree reaches a parser
  → Constraint: parsers read a `map[string]any` tree, so per-family state must be keyed by the family string the tree already uses.
- [ ] `docs/architecture/core-design.md` - reactor and session boundaries
  → Decision: enforcement stays in the session layer, and reconnect timing stays in the peer layer. The fix must not move either.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4486.md` - `Cease` subcode 1, Maximum Number of Prefixes Reached
  → Constraint: subcode 1 is sent per session, but the offending family is carried in the NOTIFICATION data. The family is therefore already known at teardown time.

**Key insights:** (minimal context to resume after compaction)
- The family loop sorts keys (`config.go`). Order is stable, so the bug is deterministic.
- YANG defaults are materialized per list entry (`schema_defaults.go`). An omitting family actively overwrites.
- `idle-timeout` is consumed at the peer layer, where no family is in scope. That is the hard part of this fix.

## Current Behavior (MANDATORY)

**Source files read:** (all read BEFORE this spec was written)
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - the per-family `prefix` container holding all five prefix leaves
- [ ] `internal/component/bgp/reactor/peersettings.go` - `PrefixMaximum` and `PrefixWarning` are maps, the other three are scalars
- [ ] `internal/component/bgp/reactor/peersettings.go` - `NewPeerSettings` sets `PrefixTeardown: true` as the direct-caller fallback
- [ ] `internal/component/bgp/reactor/config.go` - `parseFamiliesFromTree` sorts family keys, then loops
- [ ] `internal/component/bgp/reactor/config.go` - `parsePrefixLimitFromFamily` writes maps for two leaves and scalars for three
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `applyPrefixCheck` reads the scalar to decide teardown
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `familyString` converts a packed `uint32` family key to the `afi/safi` string
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `buildPrefixNotification` already receives the offending family key
- [ ] `internal/component/bgp/reactor/session_read.go` - wraps the sentinel error without any family information
- [ ] `internal/component/bgp/reactor/session.go` - `ErrPrefixLimitExceeded` is a bare sentinel
- [ ] `internal/component/bgp/reactor/peer_run.go` - reads `PrefixIdleTimeout` for the reconnect backoff
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` - copies `PrefixTeardown` from a template for dynamic peers
- [ ] `internal/component/config/schema_defaults.go` - `applyChildDefault` fills list entries and non-presence containers
- [ ] `internal/component/bgp/reactor/config_test.go` - the three existing single-family parser tests

**Behavior to preserve:** (unless the user explicitly said to change it)
- The family key sort in `parseFamiliesFromTree` stays. It keeps OPEN capability ordering deterministic.
- A missing or zero `prefix maximum` stays a hard config error (`config.go` and `config.go`).
- `warning` still defaults to 90 percent of `maximum` (`config.go`).
- `warning` greater than or equal to `maximum` stays a config error (`config.go`).
- The NOTIFICATION stays `Cease` with subcode `NotifyCeaseMaxPrefixes`, carrying AFI, SAFI, and count in 7 bytes (`session_prefix.go`).
- The JSON key `prefix-updated` keeps its current shape, one string per peer (`internal/component/bgp/plugins/cmd/peer/peer.go`).
- `plugin.PeerInfo.PrefixUpdated` stays a single string (`internal/component/plugin/types_bgp.go`).
- Warn-only mode still drops excess NLRI instead of installing it (`session_prefix.go`).
- The exponential backoff shape for repeated prefix teardowns stays, capped at one hour (`peer_run.go`).

**Behavior to change:** (only what the user asked for)
- `teardown` is honored per family instead of per peer.
- `idle-timeout` is honored per family instead of per peer.
- `updated` is stored per family instead of per peer.
- A family that omits a leaf no longer overwrites another family's explicit value.
- The comment at `config.go` stops claiming per-family behavior that the code does not deliver.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config text, parsed to a `map[string]any` tree, with YANG defaults materialized by `ApplyDefaults` (`schema_defaults.go`).
- Format at entry: nested maps. Every leaf value is a string, including booleans and integers.

### Transformation Path
1. `ApplyDefaults` fills `teardown` and `idle-timeout` into every family entry that lacks them (`schema_defaults.go`).
2. `parseFamiliesFromTree` collects family keys, sorts them, and loops (`config.go`).
3. `parsePrefixLimitFromFamily` writes two maps and three scalars (`config.go` through `config.go`).
4. `PeerSettings` reaches the session and the peer.
5. `applyPrefixCheck` reads the scalar teardown flag on overflow (`session_prefix.go`).
6. On teardown, `session_read.go` returns the sentinel error with no family attached.
7. `peer_run.go` reads the scalar idle-timeout to size the reconnect delay.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → PeerSettings | string map values coerced in `parsePrefixLimitFromFamily` | Yes, read `config.go` |
| PeerSettings → Session | struct field read in `applyPrefixCheck` | Yes, read `session_prefix.go` |
| Session → Peer (teardown cause) | `ErrPrefixLimitExceeded` sentinel, no family payload | Yes, read `session.go` and `session_read.go` |
| PeerSettings → dynamic peer | field copy from template | Yes, read `reactor_dynamic.go` |
| Reactor → plugin JSON | `plugin.PeerInfo.PrefixUpdated`, key `prefix-updated` | Yes, read `types_bgp.go` and `peer.go` |
| Reactor → report bus | staleness raise and clear from `PrefixUpdated` | Yes, read `reactor_peers.go` |

### Integration Points
- `familyString` (`session_prefix.go`) already bridges the packed `uint32` key to the `afi/safi` string used by the maps. The teardown branch is a cold path, so the conversion cost is permitted there.
- `prefixConfigLookup` (`session_prefix.go`) is the existing per-family lookup pattern. New per-family reads MUST follow it, and MUST NOT invent a second one.
- `buildPrefixNotification` (`session_prefix.go`) already receives the offending family key. The family is available at the teardown decision point.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Enforcement stays in the session, reconnect timing stays in the peer. Only the carried value changes shape. |
| No unintended coupling (components stay isolated) | Open | Plumbing the family to `peer_run.go` adds one fact to a session-to-peer error. Design must keep it minimal. |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `PrefixMaximum` keying and the `familyString` bridge. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Teardown is a cold path. No hot-path allocation is added. |
| Registration over hardcoding, per `ai/rules/plugins.md`. New commands, views, families, and handlers register themselves. The core discovers them. No per-feature field, switch case, or factory is added to a core or shared package | Yes | No new command, view, or family. Families already come from the registry via `family.LookupFamily` (`config.go`). |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Only `session_prefix.go` reads `PrefixTeardown`, and only `peer_run.go` reads `PrefixIdleTimeout` | Original brief | Fix misses a caller and leaves a stale scalar read | Grep, already run | **broken**. `PrefixTeardown` is also copied at `reactor_dynamic.go` and logged at `session_prefix.go`. `PrefixIdleTimeout` is read only at `peer_run.go` and `peer_run.go`, plus set in `session_prefix_test.go` |
| A-2 | `PrefixUpdated` has a narrow surface, so making it per-family is cheap | Field sits in the same three-line block | Scope grows into JSON and report-bus surfaces | Grep for every reader | **broken**. Five readers: `reactor_api.go`, `reactor_api.go`, `reactor_peers.go`, `session_prefix.go`, `peer.go`. It also crosses to `plugin.PeerInfo` (`types_bgp.go`) |
| A-3 | No `.ci` or `.conf` fixture sets these leaves on one family and relies on peer-wide effect | Grep of `test/` for `teardown` and `idle-timeout` | A green fixture is silently asserting the bug, and the fix turns it red | Grep, already run | **confirmed**. The only BGP prefix fixture is `test/interop/scenarios/45-max-prefix-cease-frr/ze.conf:19`, single-family `ipv4/unicast` with `idle-timeout 1` |
| A-4 | No existing unit test asserts cross-family prefix enforcement | Grep of `config_test.go` and `session_prefix_test.go` | Duplicate or conflicting coverage | Grep, already run | **confirmed**. All three parser tests declare only `ipv4/unicast` (`config_test.go`, `:1853`, `:1867`) |
| A-5 | YANG defaults reach every family entry, so a family that omits the leaf still carries `teardown true` | `schema_defaults.go` list recursion, non-presence container fill at `:62-79` | The overwrite only happens on explicit disagreement, which narrows the bug but does not remove it | Read `ApplyDefaults`, then a parser test with one family omitting the leaf | **confirmed** by reading. Still needs the test to pin it |
| A-6 | An absent map key is a safe default for `teardown` | Assumption to challenge, not a belief | A `map[string]bool` miss returns `false`, which means warn-only, which is the **less** safe direction | Unit test driving the accessor with an unconfigured family | unvalidated. Treat as a fail-closed hazard, see R-1 |
| A-7 | `idle-timeout 0` means no reconnect, as the YANG description states | `ze-bgp-conf.yang` says "0 = no reconnect" | The documented contract and the code disagree, and one of them is a separate defect | Read `peer_run.go` in full, then test the zero case | unvalidated. A read of `peer_run.go` suggests zero selects the **normal** reconnect path, not "no reconnect". Settle this before you write the AC-4 test |
| A-9 | The tests this fix must edit are free to edit | Assumption to challenge. `session_prefix_test.go` looked like ordinary test code | Three functions carry `RFC requirement:` tags, and the `rfc-tagged-test` hook blocks any behavior change to them | Grep `session_prefix_test.go` for `RFC requirement:`, then map each tag to its enclosing function | **broken**. Tagged functions are `TestPrefixWarningThreshold` (tag at `:89`), `TestPrefixExceedTeardown` (tags at `:107`, `:110`, `:114`), and `TestPrefixExceedDrop` (tag at `:144`). `TestPrefixExceedDrop` writes `ps.PrefixTeardown = false` at `:149`, which this fix must change. See R-7 |
| A-8 | The family that triggers teardown is knowable where the reconnect delay is chosen | `buildPrefixNotification` receives the family key (`session_prefix.go`) | Per-family `idle-timeout` is not implementable without new plumbing | Trace the error path from `session_prefix.go` to `peer_run.go` | **broken as written**. The family is known at the decision point but is dropped. `ErrPrefixLimitExceeded` (`session.go`) carries no family, and `session_read.go` adds none |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A `map[string]bool` for teardown fails **open**. A miss yields `false`, which disables protection for any family whose key is absent or misspelled | A family overflows and no NOTIFICATION is sent | Never read the map directly. Add an accessor whose absent case returns enabled. Drive the accessor from the session entry point, per `ai/rules/evidence.md` |
| R-2 | Dynamic peers built from a template share the template's map instead of owning a copy. One peer's parse or mutation then leaks into siblings | Two dynamic peers change enforcement together | Copy the maps in `reactor_dynamic.go`, do not alias them. Add a test with two dynamic peers |
| R-3 | Plumbing the family into the teardown error touches the session-to-peer error path, which other reconnect logic also reads | `errors.Is` checks elsewhere stop matching | Keep the sentinel matchable. Carry the family beside it, never in place of it. Grep every `errors.Is` on the reconnect path first |
| R-4 | Making `PrefixUpdated` per-family changes an external JSON surface and the staleness alarm | `prefix-updated` shape changes, or a stale peer stops alarming | Store per family, aggregate at the boundary. Pick the **oldest** date so the alarm stays conservative |
| R-5 | Scope creep from two leaves to three, plus new error plumbing, turns a small fix into a wide one | The diff grows past the parser and the session | Land teardown first, with its tests. Then idle-timeout. Then `updated`. Each phase is independently verifiable |
| R-6 | The fix is correct but the test still cannot see it, because it asserts aggregate behavior | The test passes against unfixed code | Mutation-verify. Revert the parser fix and confirm the new test goes red, per `ai/rules/testing.md` |
| R-7 | The `rfc-tagged-test` hook BLOCKS the edit this fix needs. `TestPrefixExceedDrop` writes `ps.PrefixTeardown = false` at `session_prefix_test.go`, and it carries an `RFC requirement: RFC4271-6.7-4 negative` tag at `:144`. Changing the field to a map forces a change inside a tagged function | A Write or Edit on `session_prefix_test.go` is rejected at exit 2, naming the tagged scope | Do NOT reword the tag and do NOT reach for `// test-relax:`, which does not satisfy this hook. Prefer changing only the shared helper `newTestPeerSettingsWithPrefix` (`session_prefix_test.go`), which sits outside every tagged function, so the tagged bodies stay untouched. If a tagged body genuinely must change, that needs Thomas's explicit approval recorded as `// rfc-test-change-approved:`, per `ai/rules/rfc-compliance.md`. Only the user can authorize it |
| R-8 | The concurrent `spec-rfcgate-4-ledger` session and this fix collide in `session_prefix_test.go` | A merge conflict, or an RFC tag that disappears | Coordinate with that session first. Re-run `make ze-rfc-index` when a tagged test moves line, because `ze-rfc-check` fails on a stale ledger |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Live BGP sessions. A wrong landing either stops a session the operator asked to keep, or keeps a session the operator asked to stop. R-1 is the dangerous direction, because it silently disables prefix protection. |
| How is it reverted? | Single commit revert. No config migration is needed, because the YANG grammar does not change. Existing configs keep parsing. |
| Who else touches this path? | `plan/spec-rfcgate-4-ledger.md` reads `PrefixTeardown` for its RFC 4486 `Cease` subcode ledger. A concurrent session for that spec is **actively editing** `internal/component/bgp/reactor/session_prefix_test.go`, and is adding `RFC requirement:` tags to the exact tests this fix must change. Coordinate with it before you touch that file. See R-7. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer config with two families disagreeing on `teardown` | → | `parsePrefixLimitFromFamily` (`config.go`) | `TestPrefixTeardownPerFamilyDisagreement` |
| Overflow on the warn-only family, session established | → | `applyPrefixCheck` (`session_prefix.go`) | `test/plugin/prefix-per-family-teardown.ci` |
| Overflow on the teardown family, `Cease` sent | → | `buildPrefixNotification` (`session_prefix.go`) | `test/plugin/prefix-per-family-teardown.ci` |
| Prefix teardown reconnect delay | → | `peer_run.go` backoff selection | `TestPrefixIdleTimeoutPerFamily` |
| Dynamic peer built from a template | → | `reactor_dynamic.go` settings copy | `TestDynamicPeerOwnsPrefixMaps` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer with `ipv4/unicast teardown true` and `ipv6/unicast teardown false`, IPv4 count exceeds its maximum | Ze stops the session. It sends a `Cease` NOTIFICATION with subcode Maximum Number of Prefixes Reached, carrying the IPv4 AFI and SAFI. |
| AC-2 | The same peer, IPv6 count exceeds its maximum | Session stays established. No NOTIFICATION is sent. Excess IPv6 NLRI is dropped and not installed. |
| AC-3 | Peer with `ipv4/unicast teardown false` and `ipv6/unicast` omitting `teardown` | IPv4 stays warn-only. IPv6 uses the YANG default of enabled. The omitting family does not change the IPv4 outcome. |
| AC-4 | Two families with different `idle-timeout` values, teardown triggered by one of them | The reconnect delay is derived from the offending family's own `idle-timeout`, not from another family's value. |
| AC-5 | The same two-family config, with the families declared in reverse order in the config text | Every per-family outcome from AC-1 through AC-4 is unchanged. Declaration order and key sort order have no effect. |
| AC-6 | A family key that was never configured reaches the teardown check | Enforcement is treated as enabled. The absent key never reads as warn-only. |
| AC-7 | Two families set different `updated` dates | Both dates are retained per family. The `prefix-updated` JSON key reports the oldest of them. The staleness alarm uses the oldest date. |
| AC-8 | Two dynamic peers instantiated from one template | Each peer owns its own prefix maps. Changing one peer's settings does not affect the other. |
| AC-9 | Source review of `config.go` around line 680 | The comment describes what the code does. No comment claims per-family behavior for a per-peer field. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures "stop the session on IPv4 overflow, warn only on IPv6", then overflows IPv4 | config text → `ApplyDefaults` → `parsePrefixLimitFromFamily` → `applyPrefixCheck` → `Cease` NOTIFICATION | `test/plugin/prefix-per-family-teardown.ci` |
| 2 | Overflows IPv6 on that same peer and expects the session to survive | config text → parser → `applyPrefixCheck` → NLRI dropped, session up | `test/plugin/prefix-per-family-teardown.ci` |
| 3 | Reads `show bgp peer` and expects the prefix staleness date to stay a single field | reactor → `plugin.PeerInfo` → `peer.go` → JSON key `prefix-updated` | `TestPrefixUpdatedAggregatesOldest` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrefixTeardownPerFamilyDisagreement` | `internal/component/bgp/reactor/config_test.go` | AC-1, AC-2. Two families with opposite `teardown` keep their own values after parse | |
| `TestPrefixTeardownOmittedFamilyDoesNotOverwrite` | `internal/component/bgp/reactor/config_test.go` | AC-3. A family carrying only the materialized default leaves the explicit family alone | |
| `TestPrefixTeardownSortOrderIndependent` | `internal/component/bgp/reactor/config_test.go` | AC-5. Reversing declaration order changes nothing | |
| `TestPrefixTeardownAbsentFamilyFailsClosed` | `internal/component/bgp/reactor/session_prefix_test.go` | AC-6, R-1. An unconfigured family reads as enabled, driven from the session entry point | |
| `TestPrefixIdleTimeoutPerFamily` | `internal/component/bgp/reactor/peer_run_test.go` | AC-4. The offending family's own timeout sizes the delay | |
| `TestPrefixIdleTimeoutZeroSemantics` | `internal/component/bgp/reactor/peer_run_test.go` | A-7. Pins whichever contract is correct after the YANG and code mismatch is resolved | |
| `TestPrefixUpdatedPerFamily` | `internal/component/bgp/reactor/config_test.go` | AC-7. Both dates survive the parse | |
| `TestPrefixUpdatedAggregatesOldest` | `internal/component/bgp/reactor/reactor_api_test.go` | AC-7. The single JSON field reports the oldest date | |
| `TestDynamicPeerOwnsPrefixMaps` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | AC-8, R-2. Template maps are copied, not aliased | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `prefix maximum` | 1 to 4294967295 | 4294967295 | 0 | N/A, uint32 ceiling |
| `prefix warning` | 0 to maximum minus 1 | maximum minus 1 | N/A | maximum, rejected at `config.go` |
| `prefix idle-timeout` | 0 to 65535 | 65535 | N/A | N/A, uint16 ceiling |
| prefix teardown backoff exponent | 1 to 60 | 60 | N/A | 61, clamped at `peer_run.go` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `prefix-per-family-teardown` | `test/plugin/prefix-per-family-teardown.ci` | Peer with IPv4 teardown and IPv6 warn-only. Overflow each family in turn. Assert `Cease` on IPv4 and a surviving session on IPv6 | |
| `prefix-per-family-parse` | `test/parse/prefix-per-family-parse.ci` | A two-family config with disagreeing `teardown` and `idle-timeout` parses without error | |

Both tests are drafted under `test/draft/` first, per `ai/rules/testing.md`.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `45-max-prefix-cease-frr` | `test/interop/scenarios/` | FRR | Existing single-family scenario must stay green. It is the regression guard for the unchanged path | |
| `46-max-prefix-per-family-frr` | `test/interop/scenarios/` | FRR | A real peer sees a `Cease` for the enforcing family, and no teardown for the warn-only family | |

## Files to Modify
- `internal/component/bgp/reactor/peersettings.go` - change three scalar fields to per-family maps, add the fail-closed teardown accessor, update `NewPeerSettings`
- `internal/component/bgp/reactor/config.go` - write per-family values in `parsePrefixLimitFromFamily`, correct the comment at line 680
- `internal/component/bgp/reactor/session_prefix.go` - read teardown per family in `applyPrefixCheck`, carry the offending family on teardown
- `internal/component/bgp/reactor/session.go` - let the prefix-limit teardown cause carry a family, keeping the sentinel matchable by `errors.Is`
- `internal/component/bgp/reactor/session_read.go` - preserve the family as it wraps the teardown cause
- `internal/component/bgp/reactor/peer_run.go` - select the reconnect delay from the offending family's timeout
- `internal/component/bgp/reactor/reactor_dynamic.go` - copy the prefix maps into dynamic peers instead of aliasing them
- `internal/component/bgp/reactor/reactor_api.go` - aggregate `updated` to the oldest date for the peer-level field
- `internal/component/bgp/reactor/reactor_peers.go` - use the aggregated date for the staleness raise and clear
- `internal/component/bgp/reactor/session_prefix_test.go` - the shared helper `newTestPeerSettingsWithPrefix` at line 661 only. Three test functions in this file are RFC-tagged, so read R-7 before you edit it

## Files to Create
- `test/draft/plugin/prefix-per-family-teardown.ci` - promoted to `test/plugin/` when green
- `test/draft/parse/prefix-per-family-parse.ci` - promoted to `test/parse/` when green
- `test/interop/scenarios/46-max-prefix-per-family-frr/` - `ze.conf`, `frr.conf`, and `check.py`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The grammar is already correct. `ze-bgp-conf.yang` and `:622` are already per-family. Only the Go storage is wrong |
| YANG validation constraints | No | Existing `boolean` and `uint16` types plus defaults are sufficient |
| YANG custom validators | No | No runtime-determined valid set is involved |
| CLI commands/flags | No | No new command. `show bgp peer` output shape is preserved |
| CLI grammar (keyword before value) | N-A | No command surface changes |
| Editor autocomplete | No | Automatic for the existing boolean and integer leaves |
| Functional test for new RPC/API | No | No new RPC. Functional coverage is the two `.ci` tests above |
| Pipe completeness | N-A | No new output surface |
| Env var registration | N-A | No leaf under `environment/` |
| Doctor check for runtime dependencies | N-A | No new file, socket, port, module, or certificate |
| Prometheus counters/metrics | Yes | Confirm the existing per-family labels stay correct. `setPrefixCountMetric` and `incrPrefixTeardownMetric` (`session_prefix.go`) are already family-labeled or peer-scoped. Record which is which |
| BGP family surface (new SAFI / capability / attribute) | No | No new family, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Behavior is corrected, not added. The YANG already advertised it |
| 2 | Config syntax changed? | No | Grammar unchanged. Verify no doc claims peer-wide teardown |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | `prefix-updated` keeps its shape |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | Grep `docs/guide/` for prefix-limit prose and state that `teardown` and `idle-timeout` are per family |
| 7 | Wire format changed? | No | The 7-byte NOTIFICATION data is unchanged |
| 8 | Plugin SDK/protocol changed? | No | `plugin.PeerInfo` keeps its field |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4486.md` and the `docs/features/rfc-status.md` row. Coordinate with `plan/spec-rfcgate-4-ledger.md` |
| 10 | Test infrastructure changed? | No | Owned by a concurrent workstream. Do not edit `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, if it claims per-family prefix limits |
| 12 | Internal architecture changed? | Yes | Record the session-to-peer teardown-cause change in the BGP subsystem doc |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | Label correctness is verified, not changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `config.go`, `peersettings.go`, `session_prefix.go`, and `peer_run.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify every prefix-limit example against the YANG and the parser |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the defect, then prove reachability
   - Tests: `TestPrefixTeardownPerFamilyDisagreement`, `TestPrefixTeardownSortOrderIndependent`
   - Files: `config_test.go`, plus the draft `.ci` skeletons
   - Verify: both tests fail against today's code, and the failure names the wrong per-family value. A test that passes now is measuring the wrong thing
2. **Phase: Resolve A-7 first** -- settle the zero-value contract before you write timeout code
   - Tests: `TestPrefixIdleTimeoutZeroSemantics`
   - Files: read `peer_run.go` in full
   - Verify: the YANG description at `ze-bgp-conf.yang` and the code agree, or the disagreement is recorded and raised. Do not encode a guess
3. **Phase: teardown per family** -- the highest-risk leaf first
   - Tests: `TestPrefixTeardownOmittedFamilyDoesNotOverwrite`, `TestPrefixTeardownAbsentFamilyFailsClosed`
   - Files: `peersettings.go`, `config.go`, `session_prefix.go`, and the shared test helper at `session_prefix_test.go`
   - Read R-7 FIRST. Absorb the field-type change in the shared helper `newTestPeerSettingsWithPrefix`, so no RFC-tagged test body changes
   - Verify: the accessor fails closed. Drive it from the session entry point, never from the helper alone
   - Verify: `session_prefix_test.go` tagged functions are byte-identical apart from line shifts. If one had to change, Thomas approved it in writing
4. **Phase: idle-timeout per family** -- includes the error plumbing
   - Tests: `TestPrefixIdleTimeoutPerFamily`
   - Files: `session.go`, `session_read.go`, `session_prefix.go`, `peer_run.go`
   - Verify: every existing `errors.Is` on the reconnect path still matches. Grep them first
5. **Phase: updated per family with boundary aggregation**
   - Tests: `TestPrefixUpdatedPerFamily`, `TestPrefixUpdatedAggregatesOldest`
   - Files: `config.go`, `reactor_api.go`, `reactor_peers.go`
   - Verify: the JSON key shape is byte-identical, and the staleness alarm uses the oldest date
6. **Phase: dynamic peers and the comment**
   - Tests: `TestDynamicPeerOwnsPrefixMaps`
   - Files: `reactor_dynamic.go`, `config.go` comment at line 680
   - Verify: two dynamic peers are independent
7. **Phase: functional and interop**
   - Tests: the two `.ci` tests, then interop scenario 46
   - Files: `test/draft/`, promoted on green, plus the new scenario directory
   - Verify: mutation-verify each `.ci`. Revert the parser fix and confirm the test goes red

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The teardown accessor returns enabled for an absent family. A bare map read anywhere on the enforcement path is a blocker |
| Correctness | The family key form matches. The maps use `afi/safi` strings, `session_prefix.go` uses a packed `uint32`. Every bridge goes through `familyString` |
| Naming | Field names keep the `Prefix` prefix and the YANG leaf spelling, per `ai/rules/config.md` |
| Data flow | The offending family reaches `peer_run.go`, and `errors.Is` on the sentinel still matches |
| Rule: `ai/rules/evidence.md` | The absent-key case denies. A zero value is never a valid-looking answer |
| Rule: `ai/rules/protocol.md` | No family's setting is silently approximated by another family's |
| Rule: `ai/rules/evidence.md` | Every citation in this spec was read, and A-1, A-2, A-8, A-9 are recorded as broken |
| Rule: `ai/rules/rfc-compliance.md` | No RFC-tagged test lost a polarity. `make ze-rfc-index` was re-run if a tagged test moved line, and `ai/RFC-REQUIREMENTS.md` is in the same commit |
| Registration over hardcoding | No per-family switch or hardcoded family list is added. Families come from `family.LookupFamily` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Three fields are per-family | `grep -n "PrefixTeardown\|PrefixIdleTimeout\|PrefixUpdated" internal/component/bgp/reactor/peersettings.go` shows map types |
| No bare map read on the enforcement path | `grep -n "PrefixTeardown\[" internal/component/bgp/reactor/` returns nothing outside the accessor |
| Comment corrected | `sed -n '678,692p' internal/component/bgp/reactor/config.go` |
| Functional coverage exists | `ls test/plugin/prefix-per-family-teardown.ci test/parse/prefix-per-family-parse.ci` |
| Tests gate the behavior | Revert the `config.go` fix, run the two `.ci` tests, confirm red, restore |
| Existing interop stays green | `make ze-interop-test INTEROP_SCENARIO=45-max-prefix-cease-frr` |
| RFC ledger current | `make ze-rfc-check` passes, and `make ze-rfc-index` output is committed |
| No RFC-tagged body changed without approval | `git diff internal/component/bgp/reactor/session_prefix_test.go` shows changes only in `newTestPeerSettingsWithPrefix`, or an approval comment is present |
| Full gate | `make ze-verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open guard | A missing family key must not disable prefix protection. This is the resource-exhaustion path, because prefix limits are the defense against a peer flooding the RIB |
| Resource exhaustion | Warn-only mode must still drop excess NLRI rather than install it (`session_prefix.go`) |
| Input validation | Family keys come from the config tree. An unknown key is already rejected at `config.go`. Confirm the new map writes cannot introduce an unvalidated key |
| Error leakage | The teardown cause now carries a family. Confirm no peer-controlled string reaches a log or a NOTIFICATION unbounded |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, go back to RESEARCH |
| A new test passes before the fix | The test asserts aggregate behavior. Rewrite it to assert per-family state |
| Lint failure | Fix inline. If architectural, go back to DESIGN |
| Functional test fails | Check the AC. Wrong AC goes to DESIGN, correct AC goes to IMPLEMENT |
| A-7 cannot be settled from the code | Raise the YANG and code mismatch with Thomas. Do not encode a guess |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The sort at `config.go` is why this bug is deterministic rather than flaky. A correct fix for one problem, stable capability ordering, converted a random-looking defect into a silent and reproducible one. Deterministic wrongness is harder to notice than intermittent wrongness.
- Materialized YANG defaults make "say nothing" an active statement. A family that omits a leaf does not abstain. It arrives with the default and overwrites a neighbor. Any future per-family leaf stored as a scalar will have the same shape of bug.
- `idle-timeout` shows a layering mismatch. The leaf is per family, but its only consumer lives at the peer reconnect layer, where no family is in scope. A per-family config leaf whose consumer is per-peer needs the family plumbed, or it needs to be a per-peer leaf. The YANG chose per family, so the plumbing is owed.
- A `map[string]bool` is a poor container for a safety flag whose safe value is `true`. The Go zero value points the wrong way. This is the `evidence.md` zero-value trap in its exact documented form.
- A field type change reaches further than its readers. It also reaches every test that SETS the field. When one of those tests is RFC-tagged, an ordinary refactor becomes a change that only Thomas can authorize. Route the change through a shared, untagged helper wherever one exists.
- A file can look well covered and still be uncovered for the property you care about. `TestPrefixPerFamilyIsolation` proves per-family counters, and its name invites the reader to assume per-family enforcement. Read what a test asserts, never what it is called.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Make all three fields per-family maps | Reject a config where two families disagree, at verify time | The YANG grammar plainly offers per-family control at `ze-bgp-conf.yang` and `:622`. The two sibling leaves in the same container are already maps at `peersettings.go` and `:368`. Rejecting would remove a capability the schema advertises, which is a config-surface narrowing. `ai/rules/protocol.md` says to reject only when the implementation **cannot** deliver exactly what the config asks. Here delivering is straightforward, so exact wins. If the implementer disagrees, the choice goes to Thomas rather than being decided in the diff |
| Add an accessor for teardown instead of reading the map directly | Read `map[string]bool` at the call site. Use a tri-state enum | A direct read fails open, which is the dangerous direction. An accessor concentrates the absent-key decision in one place that a test can drive. A tri-state is more faithful but adds a type for one leaf, and the accessor already removes the hazard |
| Carry the offending family beside the sentinel error, not instead of it | Add a new error type. Store the family on the Peer struct at teardown | `errors.Is(err, ErrPrefixLimitExceeded)` at `peer_run.go` must keep matching. A replacement type would break every existing matcher. A field on Peer would add cross-goroutine state to a path that already documents single-goroutine access at `session_prefix.go` |
| Store `updated` per family, aggregate to the oldest at the boundary | Keep `updated` a peer scalar. Change the JSON to a per-family object | Storage matches the YANG. Aggregation preserves the external `prefix-updated` shape at `peer.go` and `types_bgp.go`. Oldest is the conservative choice, because it keeps the staleness alarm firing when any family is stale |
| Keep the family key sort | Iterate the map directly, now that order no longer decides the winner | The sort still serves OPEN capability ordering, which `config.go` documents. Removing it would trade one determinism bug for another |

## Known Limitations
- RFC 4486 NOTIFICATION still names one family, the one that overflowed first. Simultaneous overflow in two families is reported as one family. That matches the wire format at `session_prefix.go` and is not changed here.
- The `updated` external surface stays one date per peer. An operator who wants per-family staleness dates needs a new output field. That is out of scope, and the deferral shard carries the row.
- A-7 can reveal a second, separate defect in the `idle-timeout 0` contract. If the YANG description and `peer_run.go` genuinely disagree, this spec fixes it only when the fix is small. Otherwise the implementer raises it with Thomas and homes it. It is never silently left.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

RFC 4486 defines `Cease` subcode 1, Maximum Number of Prefixes Reached. The
enforcing code is `applyPrefixCheck` (`session_prefix.go`) and
`buildPrefixNotification` (`session_prefix.go`). Keep the existing subcode
comment accurate when the teardown decision becomes per family. Coordinate the
`docs/features/rfc-status.md` row with `plan/spec-rfcgate-4-ledger.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate, `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`. A-6 and A-7 are open
- [ ] Deferral shard resolved: no live row without a destination
- [ ] Each new test mutation-verified: revert the fix, confirm red, restore

### Quality Gates
- [ ] `make ze-lint-changed` clean
- [ ] `make ze-race-reactor` passes, because the teardown cause path touches session-to-peer state
- [ ] `make ze-ste-review-changed` clean for this spec and any doc edited
- [ ] No bare map read of the teardown flag outside its accessor

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (scenario 46, plus 45 kept green)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-fixit-bgp-per-family-prefix-enforcement.md` only (commit A preserves the spec in history)
