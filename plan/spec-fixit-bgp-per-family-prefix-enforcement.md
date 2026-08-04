# Spec: fixit-bgp-per-family-prefix-enforcement

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 7/7, closing. Nothing is blocked |
| Deferral shard | `plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md` |
| Updated | 2026-08-04 |

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
| A-6 | An absent map key is a safe default for `teardown` | Assumption to challenge, not a belief | A `map[string]bool` miss returns `false`, which means warn-only, which is the **less** safe direction | Unit test driving the accessor with an unconfigured family | **broken**, exactly as R-1 predicted. An absent key is NOT safe. `PeerSettings.PrefixTeardownFor` (`peersettings.go`) is the single reader and returns `true` for a miss; the enforcement path never indexes the map. Proven by `TestPrefixTeardownAbsentFamilyFailsClosed` and `TestPrefixTeardownNilMapFailsClosed`, and by mutation `accessor-fails-open`, which makes both red plus `TestPrefixLimitConfigTeardownDefault` |
| A-7 | `idle-timeout 0` means no reconnect, as the YANG description states | `ze-bgp-conf.yang` says "0 = no reconnect" | The documented contract and the code disagree, and one of them is a separate defect | Read `peer_run.go` in full, then test the zero case | **broken, then FIXED**. `prefixReconnectDecision` returned `ok=false` for zero and `Peer.run` took its NORMAL backoff, so the peer reconnected, re-exceeded the maximum and flapped. Thomas ruled on 2026-08-03: **0 stays down, plus an explicit opt-in**. Landed as `PrefixReconnectFor` (`peersettings.go`), `prefixReconnectDecision` and `holdDownAfterPrefixTeardown` (`peer_run.go`), and the `prefix { reconnect ...; }` leaf. `TestPrefixIdleTimeoutZeroSemantics` is corrected, not deleted. CLOSED |
| A-9 | The tests this fix must edit are free to edit | Assumption to challenge. `session_prefix_test.go` looked like ordinary test code | Three functions carry `RFC requirement:` tags, and the `rfc-tagged-test` hook blocks any behavior change to them | Grep `session_prefix_test.go` for `RFC requirement:`, then map each tag to its enclosing function | **broken**. Tagged functions are `TestPrefixWarningThreshold` (tag at `:89`), `TestPrefixExceedTeardown` (tags at `:107`, `:110`, `:114`), and `TestPrefixExceedDrop` (tag at `:144`). `TestPrefixExceedDrop` writes `ps.PrefixTeardown = false` at `:149`, which this fix must change. See R-7 |
| A-10 | The `errors.As` recovery works through the production double wrap, not only through a test-built error | `session_read.go` wraps with `fmt.Errorf("%w: %w", ErrConnectionClosed, cause)` | AC-4 would be proven only against a hand-built error, and the daemon would silently take the normal backoff | Interop scenario 45, which sets `idle-timeout 1` and asserts recovery | **confirmed** end to end. Scenario 45 passes, and its recovery step only completes if `errors.As` recovered the family and `PrefixIdleTimeoutFor("ipv4/unicast")` returned 1 |
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

| Entry Point | → | Feature Code | Test | Status |
|-------------|---|--------------|------|--------|
| Peer config with two families disagreeing on `teardown` | → | `parsePrefixLimitFromFamily` (`config_prefix.go`) | `TestPrefixTeardownPerFamilyDisagreement`, `test/parse/prefix-per-family-parse.ci` | green |
| Overflow on the warn-only family, session established | → | `applyPrefixCheck` (`session_prefix.go`) | `test/plugin/prefix-per-family-teardown.ci`, `TestPrefixTeardownPerFamilyEnforcement` | green, and RED against a daemon built with the parser reverted |
| Overflow on the warn-only family, the excess NLRI reaches no RIB | → | `applyPrefixCheck` returning `drop=true` (`session_prefix.go`), consumed by `processMessage` (`session_read.go`) before plugin delivery | `test/plugin/prefix-warn-only-drops-nlri.ci` | green, and RED against a daemon whose warn-only branch returns `nil, false` |
| Overflow on the teardown family, `Cease` sent | → | `buildPrefixNotification` (`session_prefix.go`) | `TestPrefixTeardownPerFamilyEnforcement`, interop `46-max-prefix-per-family-frr` | green |
| Prefix teardown reconnect delay | → | `prefixReconnectDecision` (`peer_run.go`) | `TestPrefixIdleTimeoutPerFamily` | green |
| Dynamic peer built from a template | → | `reactor_dynamic.go` settings copy | `TestDynamicPeerOwnsPrefixMaps` | green |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Proven by |
|-------|-------------------|-------------------|-----------|
| AC-1 | Peer with `ipv4/unicast teardown true` and `ipv6/unicast teardown false`, IPv4 count exceeds its maximum | Ze stops the session. It sends a `Cease` NOTIFICATION with subcode Maximum Number of Prefixes Reached, carrying the IPv4 AFI and SAFI. | `TestPrefixTeardownPerFamilyEnforcement/teardown_family_stops_the_session` asserts the subcode and the AFI/SAFI bytes. `TestPrefixTeardownPerFamilyDisagreement` asserts the parse. Mutation `enforcement-ignores-family` turns both red |
| AC-2 | The same peer, IPv6 count exceeds its maximum | Session stays established. No NOTIFICATION is sent. Excess IPv6 NLRI is dropped and not installed. | `TestPrefixTeardownPerFamilyEnforcement/warn-only_family_keeps_the_session` drives a real MP_REACH UPDATE and asserts `drop=true` from `checkPrefixLimits`. The session-stays-up half is proven in a running daemon by `test/plugin/prefix-per-family-teardown.ci` and against FRR by interop 46. The "not installed" half is proven by `test/plugin/prefix-warn-only-drops-nlri.ci`, which reads `show bgp rib received count` through an external plugin after an over-limit UPDATE, with an in-limit control route from the PEER so a zero cannot read as a pass. Mutation `warn-only-installs-excess` makes it FAIL with `count=4, maximum is 2`. **Read the family carefully:** the daemon test's warn-only family is `ipv4/unicast`, not the `ipv6/unicast` this AC names. The IPv6 half stays unit-proven, by the MP_REACH subtest of `TestPrefixTeardownPerFamilyEnforcement`. The dropping code is family-agnostic (`applyPrefixCheck` takes the family key as an argument), so the daemon test proves the mechanism and the unit test proves it for this AC's own family |
| AC-3 | Peer with `ipv4/unicast teardown false` and `ipv6/unicast` omitting `teardown` | IPv4 stays warn-only. IPv6 uses the YANG default of enabled. The omitting family does not change the IPv4 outcome. | `TestPrefixTeardownOmittedFamilyDoesNotOverwrite`. Mutation `parser-last-family-wins` turns it red |
| AC-4 | Two families with different `idle-timeout` values, teardown triggered by one of them | The reconnect delay is derived from the offending family's own `idle-timeout`, not from another family's value. | `TestPrefixIdleTimeoutPerFamily` drives `prefixReconnectDecision` (`peer_run.go`), the function `run()` calls. Mutation `reconnect-ignores-family` turns it red |
| AC-5 | The same two-family config, with the families declared in reverse order in the config text | Every per-family outcome from AC-1 through AC-4 is unchanged. Declaration order and key sort order have no effect. | `TestPrefixTeardownSortOrderIndependent` covers both assignments of the pair. `TestPrefixTeardownPerFamilyEnforcement/the_choice_follows_the_family,_not_the_key_order` does the same at the session. `test/parse/prefix-per-family-parse.ci` declares `ipv6/unicast` first |
| AC-6 | A family key that was never configured reaches the teardown check | Enforcement is treated as enabled. The absent key never reads as warn-only. | `TestPrefixTeardownAbsentFamilyFailsClosed` and `TestPrefixTeardownNilMapFailsClosed`, both driven from `checkPrefixLimits`, the session entry point. Mutation `accessor-fails-open` turns both red |
| AC-7 | Two families set different `updated` dates | Both dates are retained per family. The `prefix-updated` JSON key reports the oldest of them. The staleness alarm uses the oldest date. | `TestPrefixUpdatedPerFamily` (storage), `TestPrefixUpdatedAggregatesOldest` (7 cases including unparseable and empty), `TestPrefixStaleUsesOldestFamily` (alarm). Mutation `aggregate-picks-newest` turns them red |
| AC-8 | Two dynamic peers instantiated from one template | Each peer owns its own prefix maps. Changing one peer's settings does not affect the other. | `TestDynamicPeerOwnsPrefixMaps`. Mutation `dynamic-peer-aliases-maps` turns it red |
| AC-9 | Source review of `config.go` around line 680 | The comment describes what the code does. No comment claims per-family behavior for a per-peer field. | The parser moved to `config_prefix.go`, whose doc comment states the per-family storage rule and why a peer-wide field would be overwritten. `config.go` no longer holds the claim |
| AC-10 | A family exceeds its maximum with `idle-timeout` unset (the YANG default) | The peer stops and STAYS down. It makes no further connection attempt, its state reads `idle-hold`, and it refuses inbound connections. | `TestPeerHeldDownAfterPrefixTeardown` runs a real peer against a real remote with a 20ms backoff, and asserts one accepted connection over 25 backoff windows plus the `idle-hold` state and the report-bus warning. It was RED against the old code, where the same log shows the overflow repeating every 20 to 40ms. `test/plugin/prefix-teardown-holds-peer-down.ci` proves it in the running daemon, mutation-verified |
| AC-11 | The same family sets `reconnect backoff` | The peer comes back on its usual connect backoff, which is the pre-2026-08-03 behavior, now explicit. | `TestPrefixReconnectExplicitModes` and `TestPrefixReconnectPerFamilyParse`, plus `test/plugin/prefix-teardown-reconnect-backoff.ci` in the running daemon. The `.ci` asserts a SECOND accepted connection and OPEN handshake, not the absence of the hold. Mutation-verified: with `PrefixReconnectFor` forced to `never`, the run ends `TIME` on an accept that never comes |
| AC-12 | A `reconnect` value contradicts `idle-timeout` in the same block | The config is refused with an error naming the family and both values. | `TestPrefixReconnectRejectsContradiction`, four cases |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures "stop the session on IPv4 overflow, warn only on IPv6", then overflows IPv4 | config text → `ApplyDefaults` → `parsePrefixLimitFromFamily` → `applyPrefixCheck` → `Cease` NOTIFICATION | `test/plugin/prefix-per-family-teardown.ci` |
| 2 | Overflows IPv6 on that same peer and expects the session to survive | config text → parser → `applyPrefixCheck` → NLRI dropped, session up | `test/plugin/prefix-per-family-teardown.ci` |
| 3 | Reads `show bgp peer` and expects the prefix staleness date to stay a single field | reactor → `plugin.PeerInfo` → `peer.go` → JSON key `prefix-updated` | `TestPrefixUpdatedAggregatesOldest` |

## 🧪 TDD Test Plan

### Unit Tests
The tests landed in NEW files rather than in `config_test.go` and
`session_prefix_test.go`. Appending to either of those risks the
`rfc-tagged-test` hook widening an edit hunk to a tagged scope (R-7), and the
new files are the natural siblings of `config_prefix.go`.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrefixTeardownPerFamilyDisagreement` | `internal/component/bgp/reactor/config_prefix_test.go` | AC-1, AC-2. Two families with opposite `teardown` keep their own values after parse | green, mutation-verified |
| `TestPrefixTeardownOmittedFamilyDoesNotOverwrite` | `internal/component/bgp/reactor/config_prefix_test.go` | AC-3. A family carrying only the materialized default leaves the explicit family alone | green, mutation-verified |
| `TestPrefixTeardownSortOrderIndependent` | `internal/component/bgp/reactor/config_prefix_test.go` | AC-5. Both assignments of the pair, so no key position wins | green, mutation-verified |
| `TestPrefixIdleTimeoutPerFamilyParse`, `TestPrefixIdleTimeoutBoundaryPerFamily` | `internal/component/bgp/reactor/config_prefix_test.go` | AC-4 parse half, plus the uint16 edges 0 and 65535 per family | green, mutation-verified |
| `TestPrefixUpdatedPerFamily` | `internal/component/bgp/reactor/config_prefix_test.go` | AC-7. Both dates survive the parse | green, mutation-verified |
| `TestPrefixMaximumStillPerFamily` | `internal/component/bgp/reactor/config_prefix_test.go` | Regression guard for the two leaves that were already maps | green |
| `TestPrefixTeardownPerFamilyEnforcement` | `internal/component/bgp/reactor/session_prefix_family_test.go` | AC-1, AC-2, AC-5 at the session. Real IPv4 body NLRI and a real MP_REACH IPv6 UPDATE | green, mutation-verified |
| `TestPrefixTeardownAbsentFamilyFailsClosed`, `TestPrefixTeardownNilMapFailsClosed` | `internal/component/bgp/reactor/session_prefix_family_test.go` | AC-6, R-1. An unconfigured family reads as enabled, driven from `checkPrefixLimits` | green, mutation-verified |
| `TestPrefixTeardownCauseNamesFamily` | `internal/component/bgp/reactor/session_prefix_family_test.go` | R-3. The cause names the family AND still matches `errors.Is(err, ErrPrefixLimitExceeded)` | green |
| `TestPrefixIdleTimeoutPerFamily` | `internal/component/bgp/reactor/session_prefix_family_test.go` | AC-4. The offending family's own timeout sizes the delay | green, mutation-verified |
| `TestPrefixIdleTimeoutZeroSemantics` | `internal/component/bgp/reactor/session_prefix_family_test.go` | A-7. Pins the behavior the code has, so the YANG mismatch is visible | green. A-7 was answered by Thomas on 2026-08-03 and this test is corrected in place, with the history in its doc comment |
| `TestPrefixReconnectDelayNonPrefixError`, `TestPrefixReconnectDelayBackoff`, `TestPrefixIdleTimeoutMaximumPerFamily` | `internal/component/bgp/reactor/session_prefix_family_test.go` | The preserved backoff shape: doubling, the one-hour cap, the counter ceiling, no overflow | green |
| `TestPrefixUpdatedAggregatesOldest`, `TestPrefixStaleUsesOldestFamily` | `internal/component/bgp/reactor/session_prefix_family_test.go` | AC-7. The single peer-level field reports the oldest date, and the alarm follows it | green, mutation-verified |
| `TestDynamicPeerOwnsPrefixMaps` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | AC-8, R-2. Template maps are copied, not aliased | green, mutation-verified |

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
| `prefix-per-family-teardown` | `test/plugin/prefix-per-family-teardown.ci` | Peer with IPv4 warn-only and IPv6 teardown. The peer overflows IPv4. The session must survive, and the decision must read `teardown=false` beside `family=ipv4/unicast`. A `reject=stderr:pattern=teardown=true` proves no family was judged with another family's value | PASS. Mutation-verified against a daemon built with the parser reverted: that run logs `teardown=true` for `family=ipv4/unicast` and sends `Cease` / "Maximum Number of Prefixes Reached" |
| `prefix-per-family-parse` | `test/parse/prefix-per-family-parse.ci` | A two-family config with disagreeing `teardown` and `idle-timeout`, declared with `ipv6/unicast` FIRST so config order is the reverse of key sort order, parses AND reads back. `ze config dump --json` must show both maximums, both idle-timeouts and both teardown values, so a tree that carried one family's values into both cannot pass | PASS. `TestCIAcceptOnlyLint` caught the first version, which asserted only `exit code 0`. The readback is the fix, not a baseline entry |
| `prefix-teardown-holds-peer-down` | `test/plugin/prefix-teardown-holds-peer-down.ci` | AC-10. The YANG default keeps the peer down. Asserts the `peer held down` line with its family and mode | PASS. Mutation-verified: with `PrefixReconnectFor` returning `backoff`, the missing line fails the run |
| `prefix-warn-only-drops-nlri` | `test/plugin/prefix-warn-only-drops-nlri.ci` | AC-2's second clause. Warn-only overflow: an external observer reads `show bgp rib received count` and requires the excess to be absent, with the PEER's in-limit control route asserted first so a zero cannot read as a pass | PASS as test 387 in 5.8s, and three more times at 5.7s, 5.9s and 6.1s. Mutation-verified: with `applyPrefixCheck`'s warn-only branch returning `nil, false`, built with `runner.TestBuildTags`, the run FAILS in 20.0s on `count=4, maximum is 2`. Three consecutive clean runs: 5.7s, 5.9s, 6.1s |
| `prefix-teardown-reconnect-backoff` | `test/plugin/prefix-teardown-reconnect-backoff.ci` | AC-11. `prefix { reconnect backoff; }` brings the peer back. `option=tcp_connections:value=2` plus an `action=send:conn=2` clause, so the assertion is a SECOND accepted connection and OPEN handshake, never the absence of the hold | PASS in 7s. Mutation-verified twice: `PrefixReconnectFor` forced to `never` ends the run `TIME` on the connection-2 clause, and a one-connection copy carrying only `reject=stderr:pattern=peer held down` fails in 1.9s, so the reject discriminates too and is kept beside the positive clause |

Every one of these was drafted under `test/draft/` and promoted on green, per `ai/rules/testing.md`. `prefix-warn-only-drops-nlri` was promoted, withdrawn on a mutation measurement that turned out to be wrong, and promoted again once the mutation was rebuilt with `runner.TestBuildTags` (review items 20 and 21).

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `45-max-prefix-cease-frr` | `test/interop/scenarios/` | FRR | Existing single-family scenario must stay green. It is the regression guard for the unchanged path | PASS, unchanged |
| `46-max-prefix-per-family-frr` | `test/interop/scenarios/` | FRR | FRR overflows a warn-only ipv4/unicast on a peer whose ipv6/unicast asks for teardown. The session must survive, and the decision must read this family's own value | PASS. Mutation-verified: with the parser reverted, the same scenario FAILS with `teardown=true` on `family=ipv4/unicast` |

## Files to Modify

Every row below was changed, the last one included. Nothing here is outstanding.

- `internal/component/bgp/reactor/peersettings.go` - three scalar fields became per-family maps. Added `PrefixTeardownFor` (fail-closed), `PrefixIdleTimeoutFor`, `OldestPrefixUpdated`. `NewPeerSettings` no longer seeds a teardown default, because the accessor states it once
- `internal/component/bgp/reactor/config.go` - `parsePrefixLimitFromFamily` moved OUT to `config_prefix.go`. The move was forced: the corrected comment pushed `config.go` to 1014 lines, past the 1000-line gate. `config.go` is now 946
- `internal/component/bgp/reactor/session_prefix.go` - `applyPrefixCheck` reads teardown per family and records the offending family key. Added `prefixLimitError` and `prefixTeardownCause`. `setPrefixConfigMetrics` uses the aggregated date
- `internal/component/bgp/reactor/session.go` - added `Session.prefixExceededFamily`, confined to the session read goroutine like `prefixCounts`
- `internal/component/bgp/reactor/session_read.go` - the teardown error now carries the family, still wrapped so `errors.Is` matches
- `internal/component/bgp/reactor/peer_run.go` - added `prefixReconnectDecision`, which picks the offending family's own timeout. The run loop calls it instead of computing the delay inline, so AC-4 has a producer a test can drive
- `internal/component/bgp/reactor/reactor_dynamic.go` - all five prefix maps are `maps.Clone`d. `PrefixIdleTimeout` and `PrefixUpdated` were never carried to dynamic peers at all before this
- `internal/component/bgp/reactor/reactor_api.go` - `PeerInfo.PrefixUpdated` is the aggregated oldest date. The `settingsEqual` exclusion now clears a map
- `internal/component/bgp/reactor/reactor_peers.go` - the staleness raise and the log use the aggregated date
- `internal/component/bgp/reactor/config_test.go` - three existing assertions read through the accessors
- `docs/features/configuration.md` - the scope column said "Per peer" for both leaves. Corrected, and a stale source anchor pointing at `peer.go` now points at `peer_run.go`
- `docs/guide/configuration.md` - the enforcement example showed a PEER-level `prefix { teardown ... }` block. No such container exists: `ze-bgp-conf.yang` has exactly one `container prefix`, inside the family list. The example would be rejected by the parser. Replaced with a two-family example
- `internal/component/bgp/reactor/session_prefix_test.go` - **DONE, approved 2026-08-03.** One line, `ps.PrefixTeardown = false` inside `TestPrefixExceedDrop`, which carries `RFC requirement: RFC4271-6.7-4 negative`. Thomas authorized it, the marker `// rfc-test-change-approved: 2026-08-03` sits above the assignment, and the right side is now `map[string]bool{"ipv4/unicast": false}`. The three assertions, the polarity and the tag are byte-identical. `go vet ./internal/component/bgp/reactor` exits 0, so the test tree builds from the repository files with no overlay

## Files to Create
- `internal/component/bgp/reactor/config_prefix.go` - the per-family prefix parser, moved out of `config.go`
- `internal/component/bgp/reactor/config_prefix_test.go` - the parser tests
- `internal/component/bgp/reactor/session_prefix_family_test.go` - the enforcement, accessor, reconnect and aggregation tests
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
| Prometheus counters/metrics | Yes | Verified, not changed. Read at the declaration (`reactor_metrics.go`), not at the call site. Family-labeled: `ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, `ze_bgp_prefix_maximum_exceeded_total`, `ze_bgp_prefix_maximum`, `ze_bgp_prefix_warning`. Peer-scoped only: `ze_bgp_prefix_teardown_total` (labels `{"peer"}`) and `ze_bgp_prefix_stale`. **Gap recorded, not closed:** the teardown counter cannot say WHICH family stopped the session, now that the decision is per family. Adding a `family` label changes an existing metric's label set and breaks dashboards, so it is a separate decision. The family IS available to the operator on the enforcement log line and in the RFC 4486 NOTIFICATION data |
| BGP family surface (new SAFI / capability / attribute) | No | No new family, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Behavior is corrected, not added. The YANG already advertised it |
| 2 | Config syntax changed? | No | Grammar unchanged. Verify no doc claims peer-wide teardown |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | `prefix-updated` keeps its shape |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes, DONE | `docs/guide/configuration.md`. The page carried a worse defect than stale prose: its enforcement example put `prefix { teardown false; idle-timeout 30; }` at PEER level. `ze-bgp-conf.yang` has exactly ONE `container prefix`, inside the family list, so that example is rejected by the parser. Replaced with a two-family example that parses, plus a Scope column |
| 7 | Wire format changed? | No | The 7-byte NOTIFICATION data is unchanged |
| 8 | Plugin SDK/protocol changed? | No | `plugin.PeerInfo` keeps its field |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4486.md` and the `docs/features/rfc-status.md` row. Coordinate with `plan/spec-rfcgate-4-ledger.md` |
| 10 | Test infrastructure changed? | No | Owned by a concurrent workstream. Do not edit `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, if it claims per-family prefix limits |
| 12 | Internal architecture changed? | Yes | Record the session-to-peer teardown-cause change in the BGP subsystem doc |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | Label correctness is verified, not changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, DONE with ONE owed command | `docs/features/configuration.md` anchored the idle-timeout and reconnect logic to `peer.go`. The logic is in `peer_run.go`, now in `prefixReconnectDecision`. Anchor corrected. **Settled 2026-08-04:** `make ze-doc-test` now PASSES and reports "No documentation drift detected", so the `ai/CODE-TO-DOCS.md is stale` red this row recorded is gone. `ai/CODE-TO-DOCS.md` is regenerated in the working tree and is NOT in this commit: it also indexes files this session does not own |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes, DONE | Every prefix-limit example was checked against the YANG. One did not parse; see row 6 |

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

### A-7 follow-up: both items stopped at the model-phase gate are now CLOSED (2026-08-03)

The A-7 fix landed and is green. Two items were stopped mid-session by the
`.claude/hooks/pretool-writeedit.py` model-phase gate
(`ai/rules/model-selection.md`). Both are done.

| Item | State | Outcome |
|------|-------|---------|
| `make ze-lint-changed` was RED on one issue | `internal/component/bgp/reactor/peersettings.go`, `PrefixReconnectMode.String`: `goconst` said the literal `"unknown"` had 3 occurrences in the package | CLOSED by the operator. `String` now returns `textbuf.StrUintStr("unknown(", uint64(m), ")")`, the shape `plugin.PeerState.String` uses. `make ze-lint-changed` reports 0 issues |
| No daemon-level test for `reconnect backoff` | AC-11 was proven by unit tests only | CLOSED. `test/plugin/prefix-teardown-reconnect-backoff.ci`. The shape in the earlier version of this row (a lone `reject=stderr:pattern=peer held down`) was MEASURED and does discriminate, but it states an absence, so it is not the assertion: the test declares `option=tcp_connections:value=2` and an `action=send:conn=2` clause, and passes only when the daemon dials a second time and completes a second OPEN handshake. The reject is kept beside it as the faster signal |

### Gate evidence for the A-7 fix (2026-08-03)

| Gate | Result |
|------|--------|
| `make ze-test-pkg PKG=./internal/component/bgp/reactor` | `ok ... 131.468s` under `-race`. Re-measured 2026-08-04 after every review fix: `ok ... 12.392s`. `make ze-race-reactor` (count=20) also `ok ... 223.111s` |
| `make ze-plugin-test` | 553/561 on 2026-08-03, with 8 load-induced timeouts that all passed in isolation. **RE-MEASURED 2026-08-04 at closure: `pass 575/575 100.0%` in 287.8s, no timeouts, with all four prefix tests green (383, 385, 386, 387).** The 2026-08-03 reds do not reproduce |
| `make ze-doc-test` | RED on 2026-08-03 on one line: `ai/CODE-TO-DOCS.md is stale`. **RE-MEASURED 2026-08-04 at closure: PASSED**, "No documentation drift detected". The three discovery indexes were regenerated in the working tree and are NOT in this commit: they index files this session does not own |
| `make ze-lint-changed` | CLEAN, 0 issues. Re-measured 2026-08-04 after every review fix: still 0 |
| TDD red | `TestPeerHeldDownAfterPrefixTeardown` failed against the pre-fix daemon, and its log shows the flap: `prefix count exceeded maximum` repeating every 20 to 40ms for five seconds |
| Mutation | With `PrefixReconnectFor` returning `backoff`, `test/plugin/prefix-teardown-holds-peer-down.ci` FAILS on the missing `peer held down` line. With it returning `never`, `test/plugin/prefix-teardown-reconnect-backoff.ci` FAILS on the second connection that never arrives |

#### One unreproduced timeout, recorded with its reproduction attempt

`prefix-teardown-reconnect-backoff` timed out once, on the FIRST full
`make ze-plugin-test` after it was promoted (test 382, 60s, the 20s authored
budget widened by `ParallelTimeoutHeadroom`). The daemon stderr in that report
ends at the teardown: no `timing: safeRunOnce returned`, no second attempt, and
no truncation marker, so the daemon wrote nothing for the remaining 57 seconds.
The output was NOT truncated by the reporter, which caps at 30 lines
(`truncateOutput`, `internal/test/runner/report.go`) and printed 19.

It did not reproduce: `make ze-plugin-test` green twice more with the test at
6.9s, and `scripts/dev/stress-repro.py` clean over 64 invocations at 4x core
oversubscription (24 at 64 burners / 16 parallel, then 40 at 128 burners / 24
parallel, that pass driving this test and its `holds-peer-down` sibling
together). This is the one case `ai/rules/completion.md` lets be recorded rather
than fixed, so the record carries the next step.

**Next step for whoever meets it again:** the stall, if it is one, sits between
`Session.Run` returning and the `timing: safeRunOnce returned` line in
`Peer.run` (`peer_run.go`), which brackets `close(p.deliverChan)`, the
`<-deliveryDone` wait, and the `p.mu.Lock()` in the `runOnce` defer. Get a
goroutine dump from the live daemon rather than another suite run: the
reporter's client output cannot show one.

- RFC 4486 NOTIFICATION still names one family, the one that overflowed first. Simultaneous overflow in two families is reported as one family. That matches the wire format at `session_prefix.go` and is not changed here.
- The `updated` external surface stays one date per peer. An operator who wants per-family staleness dates needs a new output field. That is out of scope, and the deferral shard carries the row.

### Two questions only Thomas could answer. Both are ANSWERED

**1. The RFC-tagged test edit (R-7). ANSWERED 2026-08-03, and applied.**

`TestPrefixExceedDrop` (`internal/component/bgp/reactor/session_prefix_test.go`)
set `ps.PrefixTeardown = false`. The field is now `map[string]bool`, so that line
had to become a one-entry map naming `ipv4/unicast`, which is the same statement
the untagged sibling `TestPrefixExceedDropWithdrawStillCounted` already carried.
The `rfc-tagged-test` hook refused the edit, correctly, because it cannot tell a
type migration from a rewrite.

Thomas approved it. The marker `// rfc-test-change-approved: 2026-08-03` sits
above the assignment and states what changed and why. The three assertions, the
polarity, and the `RFC requirement: RFC4271-6.7-4 negative` tag are untouched.

The overlay is gone with it. Every gate below reads the repository files:
`go vet ./internal/component/bgp/reactor` exits 0, and
`make ze-test-pkg PKG=./internal/component/bgp/reactor` is green under `-race`
with no `-overlay` on the command line.

**2. A-7: `idle-timeout 0`. ANSWERED 2026-08-03, and landed.**

Thomas ruled: **`0` means STAY DOWN, and a distinct explicit way to say
"reconnect on normal backoff" is added so both intents are expressible.**

| Decision | Where it lives |
|----------|----------------|
| `idle-timeout 0` (the YANG default) keeps the peer down | `PeerSettings.PrefixReconnectFor` (`peersettings.go`) resolves it to `PrefixReconnectNever` |
| The decision is separated from the waiting | `prefixReconnectDecision` (`peer_run.go`) is pure over settings and error; `Peer.run` executes it |
| "Stays down" is enforced in the run loop | `holdDownAfterPrefixTeardown` (`peer_run.go`) blocks on a select over the peer context and `inboundNotify`, so it cannot spin, and `Peer.run` returns when it does |
| The explicit opt-in | `prefix { reconnect never \| backoff \| timer; }` (`ze-bgp-conf.yang`) |
| Operator visibility | `PeerStateIdleHold` (shown as `idle-hold`), a `prefix-hold` report-bus warning naming the family, and the `peer held down` log line carrying `family=` and `reconnect=never` |

### Migration (BEHAVIOR CHANGE)

Every peer whose config never mentions `idle-timeout` changes behavior. Before,
a prefix-limit teardown reconnected after the normal 5 to 60 second backoff,
re-exceeded the maximum, and flapped. Now the peer stays down until an operator
recreates it: change that peer's config and commit (`peerSettingsEqual` in
`reactor_api.go` removes and re-adds a peer whose settings changed), or delete
and add the peer. Cisco and Juniper both hold the peer down for the same event.

Operators who relied on the old behavior write `prefix { reconnect backoff; }`
on the family. A peer that already sets `idle-timeout N` with N above zero is
unaffected: the absent `reconnect` leaf derives `timer` from it, which is proven
by `TestPrefixReconnectDefaults`.

Nothing in `test/` depended on the old behavior. The only fixture with a prefix
teardown is `test/interop/scenarios/45-max-prefix-cease-frr`, which sets
`idle-timeout 1` and therefore takes the timer path unchanged. One TEST did
encode the defect: `TestPrefixIdleTimeoutZeroSemantics` asserted "zero declines
the prefix backoff" because it was written to PIN the disagreement while A-7 was
open. It is corrected in place, with the history in its doc comment, not deleted.

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
- [ ] Every A-N confirmed or broken, none `unvalidated`. A-6 and A-7 are both closed
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

---

## Implementation Summary

### What Was Implemented

- `PeerSettings.PrefixTeardown`, `PrefixIdleTimeout` and `PrefixUpdated`
  (`peersettings.go`) became `map[string]bool`, `map[string]uint16` and
  `map[string]string`, keyed by the `afi/safi` string. `PrefixReconnect
  map[string]PrefixReconnectMode` joins them.
- Three accessors state the absent-key answer once: `PrefixTeardownFor` returns
  ENABLED for a family that configured nothing, `PrefixReconnectFor` returns
  `never`, and `OldestPrefixUpdated` aggregates to the oldest date. Nothing on
  the enforcement path indexes a prefix map directly.
- `parsePrefixLimitFromFamily` moved out of `config.go` into `config_prefix.go`
  and writes every leaf under `familyKey`. The move was forced by the 1000-line
  file gate; `config.go` is now under it.
- The teardown error carries the offending family. `prefixLimitError`
  (`session_prefix.go`) wraps `ErrPrefixLimitExceeded`, so every existing
  `errors.Is` on the reconnect path still matches, and `prefixReconnectDecision`
  (`peer_run.go`) reads that family's own `idle-timeout` and `reconnect`.
- `idle-timeout 0`, the YANG default, now HOLDS THE PEER DOWN.
  `holdDownAfterPrefixTeardown` (`peer_run.go`) parks the peer on a blocking
  select, refuses inbound connections, sets `PeerStateIdleHold` (`idle-hold`),
  and raises a `prefix-hold` report-bus warning naming the family. The
  `prefix { reconnect never | backoff | timer; }` leaf makes both intents
  expressible.
- `reactor_dynamic.go` clones all five prefix maps rather than aliasing the
  template, so two dynamic peers built from one group are independent.

### Bugs Found/Fixed

- The parse defect itself: the last family in sorted key order governed all
  families for three leaves. Covered by `TestPrefixTeardownPerFamilyDisagreement`,
  `TestPrefixTeardownOmittedFamilyDoesNotOverwrite` and
  `TestPrefixTeardownSortOrderIndependent`.
- A-7, found on the way: `idle-timeout 0` took the NORMAL connect backoff, so a
  peer stopped for flooding came back, re-exceeded its maximum, and flapped every
  20 to 40ms. Covered by `TestPeerHeldDownAfterPrefixTeardown` and
  `test/plugin/prefix-teardown-holds-peer-down.ci`.
- A doc example that could not parse: `docs/guide/configuration.md` showed
  `prefix { teardown ...; }` at PEER level. `ze-bgp-conf.yang` has exactly one
  `container prefix`, inside the family list.
- Dynamic peers never carried `PrefixIdleTimeout` or `PrefixUpdated` at all
  (`reactor_dynamic.go`, `buildDynamicPeerSettings`). Covered by
  `TestDynamicPeerOwnsPrefixMaps`.

### Documentation Updates

- `docs/features/configuration.md`: the Scope column read "Per peer" for
  `teardown` and `idle-timeout`; both are per family. Added the `reconnect` row
  and the stay-down behavior. The anchor
  `<!-- source: internal/component/bgp/reactor/peer.go -- idle-timeout and
  reconnect logic -->` named the wrong file and now names `peer_run.go`,
  `prefixReconnectDecision` and `holdDownAfterPrefixTeardown`.
- `docs/guide/configuration.md`: replaced the peer-level example that the parser
  rejects with a two-family example that parses, and added
  "After a prefix teardown, the peer stays down".
- `make ze-doc-test`: PASSED. "No documentation drift detected". The
  `ai/CODE-TO-DOCS.md is stale` red the spec recorded on 2026-08-03 is gone.

### Deviations from Plan

- The spec planned to put the new tests in `config_test.go` and
  `session_prefix_test.go`. They landed in new sibling files
  (`config_prefix_test.go`, `session_prefix_family_test.go`,
  `peer_prefix_hold_test.go`) so an edit hunk could never widen into an
  RFC-tagged scope. See R-7.
- `docs/comparison.md` was listed as a Yes in the Documentation checklist. It is
  NOT edited: it claims nothing about per-family prefix limits, and the
  uncommitted edits it carries belong to other concurrent work.
- Scope grew by one YANG leaf (`reconnect`) and one peer state
  (`PeerStateIdleHold`), both forced by Thomas's A-7 ruling on 2026-08-03.
- **Three files ride along that this spec did not author.**
  `peer_settings_apply.go` (new), `peer_settings_swap_test.go` (new) and
  `peer_forward_facts.go` belong to a separate reload hot-swap workstream sharing
  this working tree. They are in the commit because they cannot be left out:
  `reactor_api.go` carries this spec's `OldestPrefixUpdated` and `PrefixUpdated`
  map changes AND calls `peerSettingsRestartReason` and
  `swapPeerSettingsJournaled`, which live in `peer_settings_apply.go`. Their unit
  proof (`peer_settings_swap_test.go`) is green inside the package run. Their
  daemon-level proof is NOT: `test/reload/reload-import-policy-no-bounce.ci`
  fails on `expect=stderr pattern not found: peer settings swapped in place`,
  with the restart branch also unlogged, so the reload reaches neither branch.
  That `.ci` is left UNCOMMITTED with its owner rather than landed red.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1: only `session_prefix.go` read `PrefixTeardown` and only `peer_run.go` read `PrefixIdleTimeout` | `PrefixTeardown` is also copied in `reactor_dynamic.go` and logged in `session_prefix.go` | Grep before the field type change | Every reader routed through an accessor |
| assumption | A-2: `PrefixUpdated` had a narrow surface | Five readers, and it crosses into `plugin.PeerInfo` (`types_bgp.go`) | Grep for every reader | Stored per family, aggregated at the boundary so the external shape is unchanged |
| assumption | A-6: an absent map key is a safe default for `teardown` | A `map[string]bool` miss returns `false`, which is warn-only, the LESS safe direction | Writing the accessor test | `PrefixTeardownFor` returns `true` on a miss; mutation `accessor-fails-open` proves it |
| assumption | A-7: `idle-timeout 0` meant no reconnect, as the YANG said | The code took the normal backoff and the peer flapped | Reading `peer_run.go` in full rather than the summary | Thomas ruled; `PrefixReconnectNever` plus the explicit `reconnect` leaf |
| assumption | A-8: the offending family was already available where the reconnect delay is chosen | It is known at the decision point and DROPPED: `ErrPrefixLimitExceeded` carried no family | Tracing session to peer | `prefixLimitError` carries it beside the sentinel |
| assumption | A-9: the tests this fix must edit were free to edit | Three functions carry `RFC requirement:` tags and the `rfc-tagged-test` hook blocks a behavior change to them | Grep for `RFC requirement:` in `session_prefix_test.go` | Change routed through the untagged helper; the one tagged line Thomas approved in writing |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Honor `teardown` per family | Done | `PrefixTeardownFor` (`peersettings.go`), read at `applyPrefixCheck` (`session_prefix.go`) | Fail-closed on an absent key |
| Honor `idle-timeout` per family | Done | `PrefixIdleTimeoutFor` (`peersettings.go`), read at `prefixReconnectDecision` (`peer_run.go`) | The offending family sizes the wait |
| Honor `updated` per family | Done | `PrefixUpdated` map plus `OldestPrefixUpdated` (`peersettings.go`) | External `prefix-updated` shape unchanged |
| Keep the family key sort | Done | `parseFamiliesFromTree` (`config.go`) | Untouched; it still orders the Multiprotocol capabilities |
| Remove the cross-family overwrite | Done | `parsePrefixLimitFromFamily` (`config_prefix.go`) | Mutation `parser-last-family-wins` turns four tests red |
| Correct the misleading comment | Done | `config_prefix.go` doc comment | `config.go` no longer holds the claim |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPrefixTeardownPerFamilyEnforcement/teardown_family_stops_the_session` | Asserts the Cease subcode and the AFI/SAFI bytes |
| AC-2 | Done | `TestPrefixTeardownPerFamilyEnforcement/warn-only_family_keeps_the_session`, `test/plugin/prefix-per-family-teardown.ci`, interop 46 | All three re-measured green |
| AC-3 | Done | `TestPrefixTeardownOmittedFamilyDoesNotOverwrite` | Red under mutation `parser-last-family-wins` |
| AC-4 | Done | `TestPrefixIdleTimeoutPerFamily`, `TestPrefixIdleTimeoutPerFamilyParse` | Drives `prefixReconnectDecision`, the function `Peer.run` calls |
| AC-5 | Done | `TestPrefixTeardownSortOrderIndependent`, `test/parse/prefix-per-family-parse.ci` | Both key assignments; the `.ci` declares `ipv6/unicast` first |
| AC-6 | Done | `TestPrefixTeardownAbsentFamilyFailsClosed`, `TestPrefixTeardownNilMapFailsClosed` | Both red under mutation `accessor-fails-open` |
| AC-7 | Done | `TestPrefixUpdatedPerFamily`, `TestPrefixUpdatedAggregatesOldest`, `TestPrefixStaleUsesOldestFamily` | Oldest wins, so the alarm stays conservative |
| AC-8 | Done | `TestDynamicPeerOwnsPrefixMaps` | All five maps cloned in `buildDynamicPeerSettings` |
| AC-9 | Done | `config_prefix.go` doc comment | The parser moved; the stale claim is gone |
| AC-10 | Done | `TestPeerHeldDownAfterPrefixTeardown`, `test/plugin/prefix-teardown-holds-peer-down.ci` | One accepted connection over 25 backoff windows |
| AC-11 | Done | `TestPrefixReconnectExplicitModes`, `TestPrefixReconnectPerFamilyParse`, `test/plugin/prefix-teardown-reconnect-backoff.ci` | The `.ci` asserts a SECOND accepted connection, not an absence |
| AC-12 | Done | `TestPrefixReconnectRejectsContradiction` (four cases), `test/parse/prefix-reconnect-invalid.ci` | The `.ci` proves `ze config validate`, the operator path, reaches the check and names the family |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every test in the Unit Tests table | Done | `config_prefix_test.go`, `session_prefix_family_test.go`, `reactor_dynamic_test.go`, `peer_prefix_hold_test.go` | 54 `TestPrefix*` / `TestDynamicPeerOwnsPrefixMaps` / `TestPeerHeldDownAfterPrefixTeardown` functions PASS |
| `prefix-per-family-teardown` | Done | `test/plugin/` | PASS as test 383 of `make ze-plugin-test` |
| `prefix-teardown-holds-peer-down` | Done | `test/plugin/` | PASS as test 385 |
| `prefix-teardown-reconnect-backoff` | Done | `test/plugin/` | PASS as test 386, 7.0s |
| `prefix-per-family-parse` | Done | `test/parse/` | PASS as test 222 of `make ze-parse-test` |
| Interop 46 | Done | `test/interop/scenarios/46-max-prefix-per-family-frr/` | PASS against FRR 10.3.1 |
| Interop 45 regression guard | Done | `test/interop/scenarios/45-max-prefix-cease-frr/` | Re-run unchanged, still PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every row of "Files to Modify" | Done | Including `session_prefix_test.go`, which the approval unblocked |
| Every row of "Files to Create" | Done | The two draft `.ci` files were promoted; `peer_prefix_hold_test.go` was added for AC-10 and AC-11, which the plan predates |

### Audit Summary
- **Total items:** 12 AC, 6 Task requirements, 7 test groups, 2 file groups
- **Done:** all of them
- **Partial:** none
- **Skipped:** none
- **Changed:** 3, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Ze honors each family's own `teardown`, so an operator who asks for warn-only on one family gets it | Interop | `make ze-interop-test INTEROP_SCENARIO=46-max-prefix-per-family-frr`: `PASS 1 scenario(s)`. FRR 10.3.1 overflows a warn-only `ipv4/unicast` on a peer whose `ipv6/unicast` asks for teardown; the session survives 20s and ze logs `family=ipv4/unicast` with `teardown=false` |
| The existing single-family teardown path is unchanged | Interop | `make ze-interop-test INTEROP_SCENARIO=45-max-prefix-cease-frr`: PASS, scenario untouched |
| A family that expresses no opinion cannot overwrite one that does | Functional + mutation | `test/parse/prefix-per-family-parse.ci` PASS. Mutation `parser-last-family-wins` (a copy of `config_prefix.go` restored to the peer-wide write, applied with `go test -overlay`) turns `TestPrefixTeardownPerFamilyDisagreement`, `TestPrefixTeardownOmittedFamilyDoesNotOverwrite`, both subtests of `TestPrefixTeardownSortOrderIndependent` and `TestPrefixIdleTimeoutPerFamilyParse` RED |
| An unconfigured family never disables prefix protection | Mutation | Mutation `accessor-fails-open` (`PrefixTeardownFor` reduced to a bare map read) turns `TestPrefixTeardownAbsentFamilyFailsClosed`, `TestPrefixTeardownNilMapFailsClosed` and `TestPrefixLimitConfigTeardownDefault` RED |
| A peer stopped for flooding the RIB does not come straight back and re-flood | Functional | `test/plugin/prefix-teardown-holds-peer-down.ci` PASS in the running daemon, and `TestPeerHeldDownAfterPrefixTeardown` accepts one connection over 25 backoff windows against a real remote |
| An operator can still ask for the old reconnect behavior | Functional | `test/plugin/prefix-teardown-reconnect-backoff.ci` PASS in 7.0s, asserting a SECOND accepted connection and a second OPEN handshake |
| Warn-only DROPS the excess route rather than installing it | Functional + mutation | `test/plugin/prefix-warn-only-drops-nlri.ci` PASSES as test 387 of `make ze-plugin-test` (575/575). It reads `show bgp rib received count` through an external `ze_api.py` observer after an over-limit UPDATE and asserts that the count never goes PAST the maximum, above a floor the peer's in-limit control route establishes first, so a zero cannot read as a pass. Mutation `warn-only-installs-excess` (`applyPrefixCheck`'s warn-only branch returning `nil, false` instead of `nil, true`, built with `runner.TestBuildTags`, run through `ZE_TEST_NO_BUILD=1 ZE_BIN=<mutant>`) makes it FAIL in 20.0s with `show bgp rib received count=4, maximum is 2`. `TestPrefixTeardownPerFamilyEnforcement` carries the unit half |
| A contradictory `reconnect` config is refused at the operator's own command | Functional + mutation | `test/parse/prefix-reconnect-invalid.ci` PASS. A real binary built with both cross-leaf rejections removed answers "configuration valid" and exits 0, so the test discriminates |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| 2026-08-04: AC-2's "dropped and not installed" has no daemon-level proof | **done, and the row is DELETED from the shard** | The gap was real for one review run and is now closed, not homed: `test/plugin/prefix-warn-only-drops-nlri.ci` proves it and its mutation proves the test. Recording a gap that no longer exists costs the next reader exactly what a false claim costs |
| 2026-08-04: the interop image's `ze` client resolves no BGP show verbs, so `Ze.rib_count` answers 0 for everyone and scenario 05 is red | deferred | `plan/spec-fixit-test-harness-fail-open-guards.md`. The wrong verb IS fixed here; the client-side resolution failure is pre-existing and the shard names where to start |
| 2026-08-04: in warn-only mode the prefix counter counts routes Ze never installs | deferred | `plan/spec-bgp-per-peer-received-counter.md`. It owns per-peer received accounting, and the offered-versus-installed choice goes to Thomas before the counter changes |
| 2026-08-04: an `updated`-only config change never reaches a running peer, so the prefix-stale warning cannot clear until restart | deferred | `plan/spec-bgp-peer-settings-reload-ignored.md`. Pre-existing and unchanged by this spec, and that spec owns exactly this surface |
| 2026-07-30: expose per-family prefix staleness dates on an operator surface | deferred | `plan/spec-bgp-peer-metric-labels.md`. The reviewer judgement this row asked for was made: the field is NOT cheap, because it adds an output field to `plugin.PeerInfo`, the plugin SDK surface. The shard is NOT removed by this closure: every row in it is live |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-bgp-per-family-prefix-enforcement-c4c78ddb-c47b-4f1a-a85d-5911d7c65455.md` |
| `review_gate.py check` | clean |
| Reviewer lenses used | Run 1, two independent subagents in parallel: logic + wiring + correctness (fail-open guards, family key form, reconnect decision, map cloning, state enum, vacuous tests), and security + RFC 4486 + operator behavior + docs. Run 2, one subagent over the fixes alone |
| Run 1 | 1 BLOCKER, 8 ISSUE, 4 NOTE across the two lenses, deduplicated to items 1-10 below |
| Run 2 | 0 BLOCKER, 1 ISSUE (item 11), 1 NOTE (item 12). It also confirmed, as asked, that the assertion inversion in `TestPrefixReconnectDelayNonPrefixError` makes the test STRICTLY STRONGER rather than editing a test to match code: the old pair sent a prefix teardown down the normal connect backoff, straight back into the RIB flood, while the test's own PREVENTS line stated the fail-closed contract |
| Run 3 | 1 BLOCKER (item 14, a false mechanism written into a fixture), 2 ISSUE and 1 NOTE, all on item 11's fix. It also proved item 13's root cause is client-side verb resolution, not the verb spelling |
| Run 4 | 0 BLOCKER, 5 ISSUE, 2 NOTE (items 14 to 19). The largest is that `test/plugin/prefix-per-family-teardown.ci` proves the DECISION, not non-installation, so AC-2's second clause was claimed and not proven. Every one is fixed or homed below |
| Run 5 | 1 BLOCKER, 1 further BLOCKER-class consequence, 2 ISSUE (items 20 and 21). It re-ran the run-4 mutation and found the recorded measurement false, which turned a described gap back into closed coverage |
| Run 6 | 0 BLOCKER, 2 ISSUE, 4 NOTE (items 22 and 23 below carry them). All are text and consistency: three comments in the promoted test still described the OLD positive control, and Pre-Commit Verification had not caught up with the promotion |
| Run 7 | 0 BLOCKER, 1 ISSUE, 1 NOTE. The ISSUE is item 24: item 23 said three tables were updated and only two were, and no wiring row anywhere reached the drop path |
| Run 8 | 0 BLOCKER, 1 ISSUE, 2 NOTE. The ISSUE is item 26: the promoted test's own header still described the EQUALITY that item 25 removed. Run 8's adversarial walk confirmed the new bound is non-vacuous on every ze-peer packing and that the flake is gone rather than made less likely |
| Run 9 | 0 BLOCKER, 1 ISSUE, 1 NOTE. The ISSUE is item 27: four more comments in the same file still asserted the one-UPDATE packing the bound exists to tolerate, one of them stating an outcome that is false on a split run |
| Run 10 | 0 BLOCKER, 2 ISSUE, 2 NOTE (items 28 and 29). It read the promoted test line by line and found a symbol that does not exist and a wrong producer for the packing nondeterminism |
| Run 11 | **0 BLOCKER, 0 ISSUE**, 2 NOTE. The gate is satisfied. It verified the drop path, the coalescer, the peer's one-write-per-route loop, `DefaultRouteMsg`, `scopeReceived` and the test's wire hex against source, and confirmed `readMessage` appears nowhere in the diff. Landing verdict: commit today |
| Run 11 NOTEs | Both folded in before the commit: one clause over-specified the split outcome (a 2+1 split leaves 1, not 2, because the count is never rolled back), and item 28's file enumeration omitted the package-level `isConnectionReset` |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The doc comment claimed a teardown naming no family "resolves through PrefixReconnectFor with an unconfigured key, which reads as never". The code did the opposite: `errors.As` failed, `ok=false`, and `Peer.run` fell through to the NORMAL connect backoff. The test carried the same wrong prose above assertions that pinned the fall-through | `prefixReconnectDecision` (`peer_run.go`), `TestPrefixReconnectDelayNonPrefixError` (`session_prefix_family_test.go`) | The CODE, not the comment: a bare `errors.Is(err, ErrPrefixLimitExceeded)` now takes the prefix path with an empty family key, which `PrefixReconnectFor` reads as never. The test's assertions follow its own stated contract. Mutation `reconnect-bare-sentinel-falls-through` turns it RED |
| 2 | ISSUE | `PrefixReconnectTimer` with a zero `idle-timeout` yields a delay of 0, and the doubling never leaves zero, so the peer reconnects at once and flaps with no backoff. `parsePrefixReconnect` rejects the pair in a config, but a `PeerSettings` built in Go reaches it and neither accessor defends it | `PrefixReconnectFor` (`peersettings.go`) | The accessor resolves that pair to `PrefixReconnectNever`. Two new subtests of `TestPrefixReconnectExplicitModes`; mutation `reconnect-timer-zero-wait` turns both RED |
| 3 | ISSUE | `test/parse/prefix-per-family-parse.ci` cannot fail for the claim in its header. Both commands read the config TREE, and the defect lived downstream of the tree, so restoring the peer-wide write leaves every assertion green. It claimed AC-5 | `test/parse/prefix-per-family-parse.ci` | The header now states what it proves (grammar plus tree round-trip) and names where AC-5 is actually proven. `reconnect` was added to the round-trip |
| 4 | ISSUE | The new `prefix { reconnect ...; }` leaf had no functional test: nothing ran `ze config validate` over it | `test/parse/` | New `test/parse/prefix-reconnect-invalid.ci`, two scenarios, each asserting the error names its family. Mutation `reconnect-accepts-contradiction` (both cross-leaf rejections removed, built into a real binary) makes `ze config validate` exit 0 and print "configuration valid", so the test goes RED |
| 20 | BLOCKER | Run 5 re-ran the mutation itself instead of trusting the recorded one, and the recorded one was WRONG. `test/plugin/prefix-warn-only-drops-nlri.ci` DOES discriminate: rebuilt with the runner's own tags (`runner.TestBuildTags`), the mutant FAILS in 20.0s with `show bgp rib received count=4`, against a clean PASS. The earlier measurement used a hand-written tag list, and that was the only difference | `test/plugin/prefix-warn-only-drops-nlri.ci`, `applyPrefixCheck` (`session_prefix.go`) | Reproduced independently before acting on it: same FAIL, same count. The test is PROMOTED out of `test/draft/`, `make ze-plugin-test` is 575/575 with it as test 387, AC-2 is claimed whole, and the deferral row that homed the phantom gap is DELETED. A recorded gap that does not exist is the mirror of the false claim run 4 caught, and it costs the next reader the same way |
| 21 | ISSUE | Run 5. Artifacts built on the bad measurement understated the evidence: AC-2's split verdict, the Goal Validation row, review items 15 and 16, the Deferrals-Resolved row, the draft header, and `check.py`'s closing comment | This spec, the deferral shard, `test/interop/scenarios/46-max-prefix-per-family-frr/check.py` | All corrected together. The draft header's "NEXT STEP: find why the count stays at 1" pointed at a puzzle that does not exist (it reads 4) and is gone; what replaced it is the tag warning, which is the real lesson |
| 22 | ISSUE | Run 6. Three comments in the promoted test still described the OLD positive control, calling ze's originated static route "the positive control ... the Adj-RIB-Out the observer counts". The observer reads `show bgp rib received count`, which is `scopeReceived` and therefore the Adj-RIB-In (`showPipeline`, `internal/component/bgp/plugins/rib/rib_pipeline.go`); a route ze originates never enters that scope. The real control is the peer's `send-default-route` | `test/plugin/prefix-warn-only-drops-nlri.ci` | All three corrected, and the reason is now stated once where it belongs: the control MUST come from the peer, and an earlier version of this test used a ze-originated route and read 0 forever. The same class as item 14, and the second time this paragraph was found wrong |
| 23 | ISSUE | Run 6. Pre-Commit Verification had not caught up with the promotion: no `Wiring Verified` row and no `AC Verified` evidence for the daemon test, so two tables disagreed with AC-2 and Goal Validation | This spec, Pre-Commit Verification | All three tables now carry the promoted test with its measured result and its mutation: `Files Exist`, `AC Verified` and `Wiring Verified`. Run 7 caught that the third had been claimed and not done, and that the MANDATORY Wiring Test table had no row reaching the drop path either; both are fixed. The run-6 NOTEs are folded in with it: AC-2's family slack is stated, the header arithmetic reads 1 to 4, "two RIB scopes" reads one, and item 18's row count is corrected |
| 24 | ISSUE | Run 7. Item 23 claimed both Pre-Commit tables carried the promoted test; only `Files Exist` and `AC Verified` did. `Wiring Verified` had no row for it, and the MANDATORY `## Wiring Test` table's warn-only row still named only the tests item 15 had established do NOT prove non-installation, so NO wiring row anywhere reached the drop path. `commit_helper.py` cannot catch this: `pre_commit_verification_gaps` requires one data row per sub-table, and every sub-table had rows | This spec, `## Wiring Test` and `### Wiring Verified` | Both rows added, each naming the producing function (`applyPrefixCheck` returning `drop=true`, consumed by `processMessage` before plugin delivery) and its measured mutation. This is the fifth finding of one class in this review, a claim the artifact does not support, and the fourth time it was a claim about DOCUMENTATION rather than about code |
| 25 | BLOCKER | Found while fixing item 24's sibling NOTE, by re-running the promoted test rather than trusting one green run: it is FLAKY. ze's own read path does NOT always merge the three announced routes into one UPDATE (`readAndProcessCoalesced`, `session_coalesce.go`; ze-peer itself sends one message per route), so the surviving Adj-RIB-In count is 1 on one run and 2 on the next, and the test asserted an EQUALITY against 1. Observed FAIL: `show bgp rib received count=2, wanted 1` | `test/plugin/prefix-warn-only-drops-nlri.ci` | The assertion is now the invariant enforcement actually promises: the count never goes PAST the maximum, above a floor the control route establishes first, so a bound cannot be met by a zero. Stable over three consecutive runs, and mutation `warn-only-installs-excess` still reddens it with `count=4, maximum is 2`. A test landed one run after being promoted would have gone red in somebody else's suite |
| 26 | ISSUE | Run 8. The spec, the deferral shard and the learned summary were all updated for item 25, and the promoted test's own header was not: it still said "the assertion is an EQUALITY", seventeen lines under the new paragraph explaining why it is a bound, and still quoted the pre-fix mutation message `count=4, wanted 1` | `test/plugin/prefix-warn-only-drops-nlri.ci` | Both lines corrected, plus the clean-run timing. The sixth finding of one class in this review: a comment that outlived the code it described (`ai/rules/stale-comments.md`). Every one was caught by a reviewer reading the artifact rather than the record |
| 27 | ISSUE | Run 9. Four comments in the promoted test still said the three routes arrive in ONE UPDATE and that NONE of them is installed, seventeen lines from the paragraph explaining that ze-peer does not always pack them. Verified against the producer: `applyPrefixCheck` fires on `current > maximum` AFTER the add, so a split burst installs the control plus one /24 and drops from the next message. One of the three IS installed on that run | `test/plugin/prefix-warn-only-drops-nlri.ci`, header plus the config, constants and assertion comments | All four rewritten to state the real property: the surviving count is 1 or 2 depending on packing, never more, and that bound is what the test asserts. The seventh finding of one class, and the belief that produced item 25's flake in the first place |
| 28 | ISSUE | Run 10. `readMessage` does not exist. The prefix-drop consumer is `Session.processMessage` (`internal/component/bgp/reactor/session_read.go`); that file's methods are `ReadAndProcess`, `readAndProcessMessage`, `processMessage` and `handleConnectionClose`, beside the package-level `isConnectionReset`. The wrong name had spread to seven sites, one of them a `<!-- source: -->` anchor in `docs/features/configuration.md` and one a code comment in `session.go` | seven sites across the test, this spec, the shard, the learned summary, the docs anchor and `session.go` | Corrected everywhere, verified by grep returning nothing. The wiring row item 24 added named a symbol no grep resolved, so the artifact meant to prove the drop path could not be checked by its reader |
| 29 | ISSUE | Run 10. The packing nondeterminism was attributed to ze-peer. ze-peer is DETERMINISTIC: `Peer.runMessageLoop` (`internal/test/peer/peer.go`) writes one UPDATE per `send-route`. The merging is ze's own `readAndProcessCoalesced` (`session_coalesce.go`), which appends consecutive ipv4/unicast NLRIs with byte-identical attributes and flushes when the read buffer drains, gated by `ze.bgp.reactor.coalesce` | The promoted test, this spec, the learned summary | Re-attributed in all three, with the env knob named. It pointed a maintainer at the wrong component and hid that the behavior is switchable. Also in the same pass: the control route is `0.0.0.0/32`, not `0.0.0.0/0` (`DefaultRouteMsg`, `internal/test/peer/message.go`), and `OVER_LIMIT` now reads "after the control route" rather than "in the one over-limit UPDATE" |
| 5 | ISSUE | Interop 46 ended with a comment claiming "the excess route is not installed" above an assertion that only re-checked `session_established`. The drop half was unproven | `test/interop/scenarios/46-max-prefix-per-family-frr/check.py` | The COMMENT was the defect and the comment is fixed. A `Ze().rib_count() <= 1` assertion was written, measured, and REMOVED again: the read it needs cannot work in the interop image (see item 13), and an upper-bound assertion over a helper that answers 0 on failure is worth less than none. The scenario now states what it proves and what it does not, and names where the drop IS proven: `TestPrefixTeardownPerFamilyEnforcement` asserts `drop=true` from `checkPrefixLimits`, which is unit-level. Run 4 was right that `test/plugin/prefix-per-family-teardown.ci` proves the DECISION and not non-installation; that claim is corrected everywhere and the remaining gap is homed |
| 6 | ISSUE | `audit-test-relaxation.py` reports `[WEAKENED] config_test.go`: an RFC-tagged file changed with no approval token | `internal/component/bgp/reactor/config_test.go` | FALSE POSITIVE, and no token was written. The authoritative gate is the pre-write hook, which scopes the check to the enclosing tagged function (`_rfc_tagged_change_err(..., tag_scope=_enclosing_tagged_scope(fp, hunks))`, `.claude/hooks/pretool-writeedit.py`). The file's two tags sit in `TestPeerRouterIDValidation` and the PATHS-LIMIT test; the three edits are in `TestPrefixLimitConfigTeardownDefault`, `...TeardownFalse` and `...IdleTimeout`, which carry no tag, so the hook did not block them. The audit script is file-scoped by its own admission and calls each finding "a candidate finding for the review". The code was fixed, not the test: the field became a map and the assertions read it through the accessor with identical polarity |
| 7 | NOTE | The docs said warn-only drops "further NLRIs for the exceeded family". The drop is per UPDATE and takes the whole message | `docs/features/configuration.md`, `docs/guide/configuration.md` | Both corrected, with a source anchor on `session_read.go` |
| 8 | NOTE | Stale symbol `prefixReconnectDelay` in two test headers and five spec rows; the `PrefixTeardown` rationale comment sat after the return in `NewPeerSettings`; `config_prefix.go` read `idle-timeout` twice | `session_prefix_family_test.go`, `peer_prefix_hold_test.go`, `peersettings.go`, `config_prefix.go` | All three corrected |
| 9 | NOTE | The `// test-relax:` token in `test/plugin/prefix-teardown-holds-peer-down.ci` needs confirming | `test/plugin/prefix-teardown-holds-peer-down.ci` | Reviewed and ACCEPTED. It is the replaced-coverage case the rule permits: the removed clause could not fail (the 5-second reconnect floor outlasts the 2-second run), the reason says that was MEASURED against a reverted daemon, and it names the test that carries the assertion instead (`TestPeerHeldDownAfterPrefixTeardown`, 25 backoff windows) |
| 11 | ISSUE | Run 2. The interop bound added for item 5 passes vacuously on a read failure: `Ze.rib_count` (`test/interop/interop.py`) answers 0 for a failed exec, a 30 second timeout and a regex miss alike, and this was the only caller in the corpus asserting an UPPER bound, so 0 read as "the excess was dropped" | `test/interop/scenarios/46-max-prefix-per-family-frr/check.py` | Correct, and it was true in the worst way: making the read raise proved the read had NEVER worked (item 13). The assertion is withdrawn rather than kept vacuous. This finding is the reason the reviewer's rule is right: a test that cannot fail is worse than an admitted gap, because it reads as coverage |
| 15 | ISSUE | Run 4. "`test/plugin/prefix-per-family-teardown.ci` proves it in a running daemon" was written against AC-2's non-installation clause. That `.ci` asserts the log line, the EOR hex and a reject; it configures no plugin and reads no RIB, so it proves the DECISION and the surviving session, never non-installation | AC-2, Goal Validation, and the run-1 fix text | Every instance of the false claim is corrected, and the gap it exposed is CLOSED rather than described: `test/plugin/prefix-warn-only-drops-nlri.ci` now reads the RIB after an over-limit UPDATE. AC-2 is claimed whole, on that test plus its mutation |
| 16 | ISSUE | Run 4 pushed back on "it cannot be read from here": the harness has a second channel to the RIB through an external plugin using `ze_api.py` | `test/interop/interop.py`, `test/plugin/prefix-warn-only-drops-nlri.ci` | The pushback was ACTED ON, not argued with. The observer plugin was built on exactly that channel, with an in-limit control route so a zero could not read as a pass. It reached PASS, and the mutation confirms it FAILS against a broken daemon. The reviewer was right on both counts, and only building it could tell |
| 17 | ISSUE | Run 4. The deferral row's diagnosis blamed `test/interop/Dockerfile.ze` build tags. Verb resolution is CLIENT-LOCAL with no daemon in the path, and the same `unknown command` reproduces on the HOST with this repo's own `bin/ze` | `plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md` | The row now carries the host reproduction and starts the next reader at `cliWireToPaths` (`internal/component/cli/client/main.go`) instead of the Dockerfile |
| 18 | ISSUE | Run 4. Every deferral row was homed at this spec's own shard, which `ai/rules/planning.md` forbids: the home must be a `plan/spec-*.md` that OUTLIVES this closure | `plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md` | Every row now names a spec that exists on disk: `spec-fixit-test-harness-fail-open-guards`, `spec-bgp-per-peer-received-counter`, `spec-bgp-peer-settings-reload-ignored`, `spec-bgp-peer-metric-labels`. Each was checked with `test -f`. A fifth row was homed at `spec-fixit-vacuous-functional-tests` and then DELETED at run 5, because the gap it recorded did not exist |
| 19 | ISSUE | Run 4. The Review Gate table recorded run 4's verdict BEFORE run 4 ran | This spec, Review Gate | Corrected, and the same mistake is not repeated for run 5: it reads `pending` until its result is in hand |
| 14 | BLOCKER | Run 3. `test/interop/scenarios/46-max-prefix-per-family-frr/ze.conf` carried a comment asserting "the instance name IS the command verb". FALSE: the verb is fixed by YANG (`internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang`), compiled by `BuildVerbCommandTree` (`internal/component/cli/client/main.go`), and the config instance name reaches only `PluginConfig.Name` (`ExtractPluginsFromTree`, `internal/component/config/loader.go`) | `test/interop/scenarios/46-max-prefix-per-family-frr/ze.conf` | Both the false comment and the config edit it justified are reverted; the instance is `bgp-rib` again, matching sibling scenario 45. This was a wrong mechanism written into a fixture, which is what the next reader trusts instead of re-deriving |
| 13 | BLOCKER | Found by fixing item 11, in the shared interop harness rather than in this spec. `Ze.rib_count` asked for `show rib status`; the verb is `show bgp rib status`. Correcting it did NOT fix the call: the `ze` client inside the interop image resolves NO BGP show verbs at all, and answers `unknown command` for both spellings. Measured four times, and `make ze-interop-test INTEROP_SCENARIO=05-routes-from-frr` is RED on `Ze RIB has 0 received routes (expected >= 3)` both before and after | `Ze.rib_count` (`test/interop/interop.py`) | PARTLY fixed, and the remainder is HOMED with its next step. The wrong verb is corrected against the producing YANG. The client-side resolution failure is pre-existing, reproduces, and reds four scenarios; the deferral shard names where to start (`cliWireToPaths` in `internal/component/cli/client/main.go`, and whether `test/interop/Dockerfile.ze` builds a binary whose `cliLoader` carries the bgp `cmd/rib` YANG). Scenario 46 no longer depends on it |
| 12 | NOTE | Run 2. In warn-only mode the prefix counter counts routes Ze never installs, so a family can stay wedged until the peer withdraws routes Ze never had | `applyPrefixCheck` (`session_prefix.go`), `processMessage` (`session_read.go`) | HOMED. Pre-existing on the AC-27 drop path, untouched here, and self-correcting on withdraw. Deciding it needs an answer this spec has no standing to give: whether a prefix limit counts what the peer OFFERED or what Ze INSTALLED. RFC 4271 Section 6.7 does not say. Row added to the deferral shard, which requires the next owner to put that question to Thomas |
| 10 | ISSUE | An `updated`-only config change never reaches a running peer, so the prefix-stale warning cannot clear until restart | `peerSettingsEqual` (`reactor_api.go`), `Reactor.AddPeer` (`reactor_peers.go`) | HOMED, not parked. Pre-existing and unchanged by this spec: the field was a peer-wide string with the same exclusion and the same AddPeer-only raise. The enforcement goal holds without it, and the fix is reload-path work in three coordinated edits. Row added to `plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md` with the diagnosis and the three edits named |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/config_prefix.go` | yes | `ls -la`, 4384 bytes |
| `internal/component/bgp/reactor/config_prefix_test.go` | yes | `ls -la`, 10947 bytes |
| `internal/component/bgp/reactor/session_prefix_family_test.go` | yes | `ls -la`, 17109 bytes |
| `internal/component/bgp/reactor/peer_prefix_hold_test.go` | yes | `ls -la`, 9546 bytes |
| `test/plugin/prefix-per-family-teardown.ci` | yes | `ls -la`, 2890 bytes |
| `test/plugin/prefix-teardown-holds-peer-down.ci` | yes | `ls -la`, 3726 bytes |
| `test/plugin/prefix-teardown-reconnect-backoff.ci` | yes | `ls -la`, 5056 bytes |
| `test/parse/prefix-per-family-parse.ci` | yes | `ls -la`, 2232 bytes |
| `test/parse/prefix-reconnect-invalid.ci` | yes | `ls -la`, 2616 bytes. Added at the review gate, finding 4 |
| `test/plugin/prefix-warn-only-drops-nlri.ci` | yes | Added at the review gate, findings 15 and 20. PASS as test 387 of `make ze-plugin-test` (575/575) |
| `test/interop/scenarios/46-max-prefix-per-family-frr/` | yes | `check.py` 3501, `frr.conf` 362, `ze.conf` 842 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-5 | Per-family enforcement at the session | `go test -run TestPrefixTeardownPerFamilyEnforcement`: PASS |
| AC-2, second clause | The excess NLRI is dropped and NOT installed | `test/plugin/prefix-warn-only-drops-nlri.ci` PASS as test 387 of `make ze-plugin-test` (575/575), re-run alone in 6.0s. Mutation `warn-only-installs-excess` FAILS it in 20.0s with `show bgp rib received count=4, maximum is 2`. Run three times green (5.7s, 5.9s, 6.1s) |
| AC-3 | An omitting family does not overwrite | `--- PASS: TestPrefixTeardownOmittedFamilyDoesNotOverwrite`, and RED under mutation `parser-last-family-wins` with the message "the explicit warn-only choice survives a defaulted sibling" |
| AC-4 | The offending family's own timeout | `--- PASS: TestPrefixIdleTimeoutPerFamily` |
| AC-6 | Fail closed on an absent family | `--- PASS: TestPrefixTeardownAbsentFamilyFailsClosed`, `TestPrefixTeardownNilMapFailsClosed`; both RED under mutation `accessor-fails-open` |
| AC-7 | Oldest date aggregation | `--- PASS: TestPrefixUpdatedAggregatesOldest`, `TestPrefixStaleUsesOldestFamily` |
| AC-8 | Dynamic peers own their maps | `--- PASS: TestDynamicPeerOwnsPrefixMaps` |
| AC-10, AC-11, AC-12 | Hold down, explicit reconnect, contradiction rejected | `--- PASS: TestPeerHeldDownAfterPrefixTeardown`, `TestPrefixReconnectExplicitModes`, `TestPrefixReconnectRejectsContradiction` |
| All | Nothing regressed | `make ze-test-pkg PKG=./internal/component/bgp/reactor` green under `-race`; 54 `TestPrefix*` functions PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Two families disagreeing on `teardown`, parsed and read back | `test/parse/prefix-per-family-parse.ci` | Read: declares `ipv6/unicast` FIRST and asserts both maximums, both idle-timeouts, both teardown values and the `reconnect` leaf through `ze config dump --json`. PASS. It proves the GRAMMAR and the tree round-trip, NOT per-family storage: both commands read the tree, and the defect lived downstream of it. The header says so; AC-5's storage proof is `TestPrefixTeardownSortOrderIndependent` |
| A `reconnect` value contradicting `idle-timeout` in the same family | `test/parse/prefix-reconnect-invalid.ci` | Read: two scenarios through `ze config validate`, each asserting exit 1 and an error naming its own family. Mutation-verified against a real binary built with both rejections removed: that binary prints "configuration valid" and exits 0 |
| Overflow on the warn-only family, session established | `test/plugin/prefix-per-family-teardown.ci` | Read: asserts `family=ipv4/unicast` beside `teardown=false`, and carries `reject=stderr:pattern=teardown=true` so no family may be judged with another's value. PASS as test 383 |
| Overflow on the teardown family, `Cease` sent | interop `46-max-prefix-per-family-frr` | Read `check.py`: waits for the FRR session, asserts the overflow line names `ipv4/unicast` with `teardown=false`, then holds 20s asserting the session is still Established. PASS |
| Prefix teardown holds the peer down | `test/plugin/prefix-teardown-holds-peer-down.ci` | Read: asserts the `peer held down` line with its family and mode. PASS as test 385 |
| `reconnect backoff` brings the peer back | `test/plugin/prefix-teardown-reconnect-backoff.ci` | Read: `option=tcp_connections:value=2` plus an `action=send:conn=2` clause, so the assertion is a SECOND accepted connection. PASS as test 386 |
| Warn-only overflow, the excess NLRI must not reach the RIB | `test/plugin/prefix-warn-only-drops-nlri.ci` | Read: an external `ze_api.py` observer waits for the PEER's in-limit control route in `show bgp rib received count` FIRST, then polls the same scope for the violation across a settle window and requires the count to stay at or under the maximum, above a floor the control establishes. PASS as test 387, re-run alone in 6.0s. Mutation `warn-only-installs-excess` FAILS it in 20.0s on `count=4, maximum is 2`, so the row is measured and not asserted |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | `PrefixTeardown` also read in `reactor_dynamic.go` and `session_prefix.go`; all readers now go through `PrefixTeardownFor` |
| A-2 | broken | Five readers plus `plugin.PeerInfo`; `OldestPrefixUpdated` keeps the external shape |
| A-3 | confirmed | The only BGP prefix fixture was `test/interop/scenarios/45-max-prefix-cease-frr/ze.conf`, single-family with `idle-timeout 1`; it takes the timer path and still PASSES |
| A-4 | confirmed | The three pre-existing parser tests declare only `ipv4/unicast`; they now read through the accessors and PASS |
| A-5 | confirmed | `TestPrefixTeardownOmittedFamilyDoesNotOverwrite` PASSES and goes RED under `parser-last-family-wins` |
| A-6 | broken | An absent key is NOT safe: `PrefixTeardownFor` returns `true` on a miss; mutation `accessor-fails-open` reddens three tests |
| A-7 | broken, then FIXED | Thomas ruled 2026-08-03: 0 stays down, plus an explicit opt-in. `PrefixReconnectFor`, `prefixReconnectDecision`, `holdDownAfterPrefixTeardown`, and the `reconnect` leaf |
| A-8 | broken as written | The family was known and dropped; `prefixLimitError` now carries it beside the sentinel, and `TestPrefixTeardownCauseNamesFamily` proves `errors.Is` still matches |
| A-9 | broken | Three RFC-tagged functions; the one tagged line that had to change carries Thomas's `// rfc-test-change-approved: 2026-08-03` marker |
| A-10 | confirmed | Interop 45 sets `idle-timeout 1` and its recovery step completes only if `errors.As` recovered the family |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features/configuration.md` Scope column reads "Per family" for `teardown` and `idle-timeout` | `ze-bgp-conf.yang` has exactly one `container prefix`, inside the family list | yes |
| `docs/features/configuration.md` anchor names `peer_run.go` | `prefixReconnectDecision` and `holdDownAfterPrefixTeardown` both exist there | yes |
| `docs/guide/configuration.md` prefix example parses | The example is two families each with their own `prefix` block, which is what `test/parse/prefix-per-family-parse.ci` proves the parser accepts | yes |
| `docs/comparison.md` (checklist row 11, "Affects daemon comparison?") | grep for `prefix` in `docs/comparison.md` finds no per-family prefix-limit claim, so no update is owed. Answer corrected from Yes to No | yes |
| `docs/functional-tests.md` (checklist row 10) | Owned by a concurrent workstream; not edited, as the checklist already stated | yes |
| `make ze-doc-test` | PASSED, "No documentation drift detected" | yes |

## Core Insight

A YANG grammar can advertise per-family control while the Go storage quietly
collapses it to one value per peer, and NOTHING in the repository can see the
gap: the parser compiles, the schema validates, every test is single-family, and
the sort that makes the wrong value deterministic is itself correct. The tell is
structural rather than behavioral. A leaf inside a list container whose Go field
is a scalar is the defect, before anyone runs it. Two sibling leaves in the same
container were already maps, which is the shape of the evidence.
