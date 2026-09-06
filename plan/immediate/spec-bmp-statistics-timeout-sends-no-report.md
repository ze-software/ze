# Spec: bmp-statistics-timeout-sends-no-report

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/3 |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.**
`internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` declares
`leaf statistics-timeout` under the BMP sender configuration, typed
`uint16` with `range "0..65535"`, `units "seconds"` and default 0, described as
"Interval for periodic statistics reports (0 = disabled)". A reader takes a
nonzero value to mean Ze sends a BMP Statistics Report to the collector at that
interval, which is the message RFC 7854 Section 4.8 defines, and takes 0 to be
the one value that turns the reports off.

**What Ze does instead.** Ze sends no Statistics Report at any value.
`(*senderSession).writeStatisticsReport`
(`internal/component/bgp/plugins/bmp/sender.go`) encodes and sends the message,
and it has no non-test caller: the only calls are in
`internal/component/bgp/plugins/bmp/sender_test.go` and
`internal/component/bgp/plugins/bmp/rfc8671_test.go`. The periodic timer the
leaf configures does not exist, so nothing would call it on a schedule. The
value is parsed and carried: `senderConfig.StatisticsTimeout`
(`internal/component/bgp/plugins/bmp/sender_config.go`) holds it, and
`behaviorOf` in the same file copies it into `senderBehavior.statistics`. That
field is compared, so a change to the leaf bounces every session of the sender
with a Peer Down and Peer Up sequence. The operator therefore pays a session
bounce for a value that changes nothing on the wire. `behaviorOf` names the
situation in its own doc comment, that the leaf is declared sender behavior and
the timer is not implemented.

**What closing it means.** The implementer chooses between building the timer
and refusing the leaf at commit the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses `vrf`. Building it is the
obviously right answer here, and it is unusually cheap, because the encoder and
the wire format already exist and are tested: the work is a per-session ticker
that gathers the counters and calls `writeStatisticsReport`. What the design
owes is which counters Ze can honestly report, since RFC 7854 Section 4.8
defines a stat type registry and Ze must not send a type it does not measure. A
refusal would leave a correct encoder with no caller, which
`ai/rules/rfc-compliance.md` treats as a behavior that reaches no wire.

**Why this is not one spec with the receiver leaf.** The other unowned BMP leaf,
`bmp/route-action`, is covered by
`plan/immediate/spec-bmp-route-action-redistribute-acts-as-monitor.md`. The two
share no code and no direction. This one is Ze as the monitored router, emitting
to a collector through the sender session. That one is Ze as the collector,
deciding what to do with routes it receives. Neither change touches the other's
files.

## Progress (2026-09-06, implemented)

The dedup change is IN SCOPE and was kept, in a smaller shape than the paused
session left. RFC 7854 Section 4.8 ends "if an SR message is transmitted, at
least one statistic MUST be carried in it", so the report needs a counter ze
honestly measures. Stat Type 13, "Number of duplicate update messages received",
is the one ze already has the machinery for: the BMP plugin hashes UPDATE bodies
per peer for Route Monitoring dedup. Counting them needs a counter, and the RFC
says RECEIVED, which needs the two directions to be told apart. Under one shared
hash space a body ze advertised back to a peer is counted as one it received,
and the Route Monitoring carrying it is suppressed. Both were defects before this
spec; both are inside the counter this spec has to be honest about.

The paused session made `dedupState` a `map[string]*peerDedup` holding two sets
and the counter. That edit reached inside
`TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession`, which carries an
RFC requirement tag, and `./le commit create` refuses a commit that changes a
tagged test without an owner-approved row in `test/rfc-changed/`. No such
approval exists, so the shape was changed rather than the test: `dedupState`
keeps its committed type and the sent direction is salted into the same hash
space (`dedupSentSalt`, `dedupKey`), with the counter in a sibling map keyed the
same way. That is one map per peer rather than two, and the tagged test compiles
byte for byte as HEAD holds it.

Everything below is complete. The interop scenario is
`test/interop/scenarios/bmp-statistics-pmacct/`, not `bmp-frr/`: `bmp-frr` reads
ze's stream with ze's own collector, which is not another implementation.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/guide/bmp.md` - the operator page for both BMP roles, including the sender leaf table and the bounce rules
  -> Decision: the four leaves that decide what a collector session carries are `route-monitoring-policy`, `route-mirroring`, `loc-rib` and `statistics-timeout`; a move in any of them bounces the peers and leaves the BMP session up.
  -> Constraint: the page stated the leaf drives no timer. That sentence is a promise to a reader and had to change in the same work as the code.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7854.md` - the Statistics Report and its stat type registry
  -> Constraint: RFC7854-x-15, "Statistics Reports should be sent periodically", was untested because ze sent none.
- [ ] `rfc/short/rfc8671.md` - the O flag on the per-peer header
  -> Constraint: RFC8671-6.2-1 was a `{gap}` because no production path emitted a Statistics Report. Building the emission path closes it, and the ledger, the extraction site and the encoder test's doc comment all had to be corrected with it.
- [ ] `rfc/short/rfc9069.md` - Loc-RIB monitoring
  -> Constraint: Section 5.6 names stat types 8 and 10 as the ones relevant to a Loc-RIB, and both count routes in the Loc-RIB itself. The BMP plugin holds no such count, so the emulated peer gets no report.

**Key insights:** (minimal context to resume after compaction)
- A report must carry at least one statistic (Section 4.8), so the design question is which counter ze can honestly report. Stat Type 13 is the only one ze measures.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/bmp/sender.go` - `writeStatisticsReport` encodes the message and clears the RFC 8671 O flag. It had no non-test caller.
- [ ] `internal/component/bgp/plugins/bmp/sender_config.go` - `parseSenderConfig` fills the leaf from its YANG default, `behaviorOf` carries it into `senderBehavior`, `applySenderConfig` compares and bounces.
- [ ] `internal/component/bgp/plugins/bmp/bmp_events.go` - `handleStructuredEvent` snapshots the config leaves under one read lock; `handleSenderUpdate` hashed UPDATE bodies for Route Monitoring dedup.
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - `BMPPlugin` holds `peerUps`, `dedupState` and `stopCh`; `runBMPPlugin` closes `stopCh` before waiting on `bp.sessions`.
- [ ] `internal/component/bgp/plugins/bmp/msg.go` - `statisticsReport` and `StatEntry`, and the codec both ends share.

**Behavior to preserve:** (unless the user explicitly said to change it)
- A change to `statistics-timeout` still bounces every monitored peer with a Peer Down/Peer Up pair and leaves the BMP session up (RFC 8671 Section 7.2).
- Route Monitoring dedup still suppresses a repeated UPDATE body for a peer, and still lets a body with different attributes through.
- The RFC 9069 Loc-RIB emulated peer is untouched.

**Behavior to change:** (only what the user asked for)
- A nonzero `statistics-timeout` now emits a Statistics Report per established peer, to every collector, at that interval.
- The two directions occupy separate hash spaces inside one per-peer set (`dedupSentSalt`), so a body ze advertised is no longer read as a repeat of one it received.
- A received UPDATE the route-monitoring policy does not stream is still hashed while a report interval is configured, so the counter is measured rather than defaulted.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `bgp bmp sender statistics-timeout`, a `uint16` in seconds delivered to the plugin as a string in the `bgp` config subtree.
- The engine delivers it on Stage-2 configure at startup and on config-apply for every reload that carries the `bgp` root.

### Transformation Path
1. `parseSenderConfig` (`sender_config.go`) fills the leaf from its YANG default when the operator deleted it.
2. `applySenderConfig` compares `behaviorOf(previous)` against `behaviorOf(current)` and returns early when nothing moved.
3. `setStatisticsTimeout` (`statistics.go`) installs the interval and starts or replaces the one ticker.
4. `statisticsLoop` waits on the ticker, the per-configuration stop channel, and the plugin's `stopCh`.
5. `sendStatisticsReports` snapshots the sender set and every `peerUps` entry under one read lock, builds one `statisticsReport` per peer, and writes them outside the lock.
6. `(*senderSession).writeStatisticsReport` clears the O flag and enqueues the encoded message on the collector's transmit queue.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine <-> Plugin | `bgp` config subtree as JSON, over the config-verify and config-apply callbacks | Yes -- `TestRFC7854StatisticsTimeoutSendsPeriodicReports` drives the callbacks rather than `applySenderConfig` |
| Plugin <-> collector | BMP over TCP, message type 1 | Yes -- `test/plugin/bmp-sender-statistics.ci` and the `bmp-statistics-pmacct` interop scenario |
| Reactor <-> Plugin | `rpc.StructuredEvent` UPDATE events feed the duplicate counter | Yes -- `TestBMPStatisticsMeasuresTheDirectionThePolicyAlsoStreams` |

### Integration Points
- `applySenderConfig` - the one apply path; the timer is installed there beside `setSenderPolicy`.
- `BMPPlugin.sessions` - the ticker goroutine joins the WaitGroup `runBMPPlugin` waits on after closing `stopCh`.
- `peerUps` - the per-peer state that already exists for Peer Up replay is what decides which peers a report describes.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The leaf reaches the timer only through `parseSenderConfig` -> `applySenderConfig` -> `setStatisticsTimeout`; the tagged wiring test drives the engine callbacks |
| No unintended coupling (components stay isolated) | Yes | Everything new is inside `internal/component/bgp/plugins/bmp`; no core or reactor package changed |
| No duplicated functionality (extends existing, does not recreate) | Yes | The encoder, the per-peer header and the transmit queue are reused; the counter reuses the dedup hasher rather than adding a second one |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `makeStatCounter` allocates one 4-byte value per report and the encoder writes into the session's scratch buffer, as every other BMP message does |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No central enumeration changed. The one new stat type is a constant inside the BMP plugin |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Stat Type 13 is the only RFC 7854 Section 4.8 type ze can report honestly | `rfc/full/rfc7854.txt` Section 4.8 lists types 0..13; types 0..6 and 11..12 are policy and loop counters the reactor holds and the BMP plugin does not, 7..10 are RIB route counts the plugin does not hold | A report would carry a type ze never measured, which a collector reads as a real zero | Read of Section 4.8 against `BMPPlugin`'s fields | confirmed |
| A-2 | The two directions must be told apart for the counter to be honest | RFC 7854 Section 4.8 says "duplicate update messages RECEIVED" | A body ze advertised back to a peer counts as one it received | `TestBMPDuplicateUpdateCountsReceivedRepeatsOnly` | confirmed |
| A-3 | pmacct decodes a Statistics Report and names stat type 13 | The `bmp-locrib-pmacct` scenario already reads ze's stream with pmacct | The interop scenario proves nothing about the stat type | The `bmp-statistics-pmacct` run of 2026-09-06, where pmacct printed `"counter_type_str": "Number of duplicate update messages received"` | confirmed |
| A-4 | A peer with no dedup entry has a measured zero rather than an unmeasured one | The detector runs for every received UPDATE while an interval is configured, whatever the policy | The report would publish a zero ze never took | `TestBMPStatisticsMeasuresUnderAPolicyThatStreamsNothing` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two tickers run after a reload and a collector reads two report streams | A collector receiving reports at a rate the interval does not explain | `setStatisticsTimeout` is the only place a ticker starts, it closes the previous stop channel, and `TestBMPStatisticsTimeoutRunsOneTickerAtATime` asserts it |
| R-2 | The counter over-reports past the per-peer hash cap | A peer churning more than `maxDedupPerPeer` distinct bodies | Past the cap a repeated body is neither suppressed nor counted, so the counter is a floor rather than an over-count; stated in `duplicateUpdate`'s doc comment |
| R-3 | The report interval floods a collector on a large peer set | One message per peer per tick | The interval is the operator's own control, which is what RFC 7854 Section 4.8 asks for; the transmit queue resets a session that stalls rather than dropping messages |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A collector reads a counter that is wrong, or reads reports at an interval nobody configured. Nothing in BGP forwarding or session handling changes: BMP is a one-way monitoring feed. The per-direction dedup split also decides which Route Monitoring messages are suppressed, so getting it wrong hides a route change from a collector |
| How is it reverted? | Single commit revert. Nothing is persisted and no collector state survives a session |
| Who else touches this path? | `plan/immediate/spec-bmp-route-action-redistribute-acts-as-monitor.md` owns the receiver leaf `bmp/route-action` and shares no file with this |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `bgp bmp sender statistics-timeout 1` committed over the plugin config-apply callback | -> | `applySenderConfig` -> `setStatisticsTimeout` -> `statisticsLoop` -> `sendStatisticsReports` -> `writeStatisticsReport` | `TestRFC7854StatisticsTimeoutSendsPeriodicReports` |
| The same leaf in a running daemon's config file, read by a collector on a socket | -> | the same chain, over a real TCP connection | `test/plugin/bmp-sender-statistics.ci` |
| The same leaf, read by pmacct | -> | the same chain, decoded by another implementation | interop scenario `bmp-statistics-pmacct` |
| `statistics-timeout 0` | -> | `setStatisticsTimeout(0)`, which starts no ticker | `TestRFC7854StatisticsTimeoutZeroSendsNoReport` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `statistics-timeout N` with N > 0, one collector connected, one BGP peer established | The collector receives one BMP Statistics Report for that peer every N seconds |
| AC-2 | `statistics-timeout 0`, the YANG default | The collector receives no Statistics Report at all |
| AC-3 | Any report ze emits | It carries at least one statistic, RFC 7854 Stat Type 13 as a 4-byte counter, and its per-peer header carries the RFC 8671 O flag as zero |
| AC-4 | A peer sends the same UPDATE body twice, then ze advertises that body to it twice | The counter reads 1: a repeat received counts, a repeat sent does not |
| AC-5 | `statistics-timeout` moves from one nonzero value to another | Exactly one report stream remains, at the new interval |
| AC-6 | `route-monitoring-policy post-policy` with a report interval configured | A received UPDATE is still hashed, so the counter is measured rather than reported as an untaken zero; no Route Monitoring is streamed for it |
| AC-7 | `statistics-timeout` set anywhere in `0..65535`, or to a value the config tree cannot deliver as a number | The interval is that many seconds; an unreadable value sends no report rather than one at an interval nobody asked for |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | commits `statistics-timeout 1` and watches a collector | config-apply -> `applySenderConfig` -> `setStatisticsTimeout` -> ticker -> `sendStatisticsReports` -> TCP | `test/plugin/bmp-sender-statistics.ci` |
| 2 | points that collector at pmacct and reads its msglog | the same path, decoded by pmacct | interop scenario `bmp-statistics-pmacct` |
| 3 | sets the leaf back to 0 and sees the reports stop | config-apply -> `setStatisticsTimeout(0)` -> the running ticker's stop channel closes | `TestRFC7854StatisticsTimeoutZeroSendsNoReport` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBMPDuplicateUpdateCountsReceivedRepeatsOnly` | `internal/component/bgp/plugins/bmp/statistics_test.go` | AC-4: a received repeat counts, a distinct body does not, a sent repeat does not | pass |
| `TestBMPStatisticsMeasuresTheDirectionThePolicyAlsoStreams` | same | AC-6: under `all`, the received direction is streamed and measured | pass |
| `TestBMPStatisticsMeasuresUnderAPolicyThatStreamsNothing` | same | AC-6: under `post-policy`, the received direction is measured and not streamed | pass |
| `TestBMPStatisticsOffMeasuresNothingUnderAPolicyThatStreamsNothing` | same | AC-6 negative: with no interval configured, that direction is not hashed at all | pass |
| `TestBMPStatisticsReportCarriesTheDuplicateCounter` | same | AC-3: one report per peer, Stat Type 13 as a 4-byte counter, O flag clear | pass |
| `TestBMPStatisticsTimeoutRunsOneTickerAtATime` | same | AC-5: one ticker per configuration, replaced on a move, stopped at zero | pass |
| `TestBMPStatisticsTimeoutParsesEveryValueTheLeafAccepts` | same | AC-7: the whole `uint16` range, and the two unreadable values | pass |
| `TestRFC7854StatisticsTimeoutSendsPeriodicReports` | same | AC-1, over the engine's own config rail, asserting TWO reports one interval apart | pass |
| `TestRFC7854StatisticsTimeoutZeroSendsNoReport` | same | AC-2, same rail, same peer, same collector | pass |
| `TestRFC8671StatisticsReportOnTheWireClearsTheOFlag` | same | AC-3: the O flag is zero on a report the emission path produced, and the L flag survives | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `statistics-timeout` | 0-65535 seconds | 65535 | N/A (0 is valid and means disabled) | 65536, which the config tree cannot deliver as a `uint16` and which installs no report |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bmp-sender-statistics` | `test/plugin/bmp-sender-statistics.ci` | An operator sets `statistics-timeout 1` and a collector reads two valid Statistics Reports, each carrying Stat Type 13 with the O flag clear | written; NOT RUN GREEN in this checkout -- every `.ci` that boots ze fails on another session's in-flight `ze-ddos-detect-conf` YANG edit, including the pre-existing `bmp-sender-route-mirroring` |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bmp-statistics-pmacct` | `test/interop/scenarios/bmp-statistics-pmacct/` | FRR (the monitored BGP peer) and pmacct (the collector) | pmacct decodes ze's Statistics Report, attributes it to the FRR peer, reads the per-peer flags as Adj-RIB-In (the RFC 8671 Section 6.2 O flag as zero), and names stat type 13 out of its own table; and it reads more than one, so the reports are periodic | pass, and RED under a reverted wiring |

## Files to Modify
- `internal/component/bgp/plugins/bmp/bmp.go` - `dedupSentSalt` and `dedupKey`, the `dedupCount` map, the two fields the ticker is held in, and the plugin's construction
- `internal/component/bgp/plugins/bmp/bmp_events.go` - `policyStreams`, `duplicateUpdate`, and the measure-versus-stream decision in `handleSenderUpdate`
- `internal/component/bgp/plugins/bmp/sender_config.go` - `applySenderConfig` installs the interval; `behaviorOf`'s comment no longer says the timer is missing
- `internal/component/bgp/plugins/bmp/rfc8671_test.go` - the two reload helpers close `stopCh` before waiting on `bp.sessions`, and the encoder test's doc comment no longer says nothing calls the encoder
- `internal/component/bgp/plugins/bmp/event_test.go`, `internal/component/bgp/plugins/bmp/sender_queue_test.go` - the `dedupState` type
- `internal/test/fixture/plugin_fixture_04_bmp.go`, `internal/test/fixture/plugin_fixture_04.go` - the `statistics` collector mode and its two registrations
- `internal/le/interoplab/bgp/names.go`, `checkers.go`, `check_extras.go` - the interop scenario and its assertions
- `docs/guide/bmp.md` - the leaf table, the message-type table, the sender behavior list and the bounce section
- `rfc/short/rfc8671.md`, `rfc/extraction/rfc8671.json` - RFC8671-6.2-1 is no longer a gap
- `plan/journal/unwired-feature.md` - the 2026-08-30 row is marked fixed
- `docs/architecture/testing/interop.md` - NOT edited, and named here because `internal/le/interoplab/bgp/names.go` declares it. The page describes how a scenario is discovered, named and asserted; this change adds one scenario through those rules and alters none of them

## Files to Create
- `internal/component/bgp/plugins/bmp/statistics.go` - the ticker, the stat types and the per-peer reports
- `internal/component/bgp/plugins/bmp/statistics_test.go` - the unit and tagged tests
- `test/plugin/bmp-sender-statistics.ci` - the functional test
- `test/interop/scenarios/bmp-statistics-pmacct/{ze.conf,frr.conf,pmbmpd.conf}` - the interop scenario
- `rfc/discrimination/rfc7854.json`, `rfc/discrimination/rfc8671.json` - the recorded reds behind the three new tags

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | `statistics-timeout` already exists in `internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang`. This spec wires the leaf that is declared, and adds none |
| YANG validation constraints | No | The leaf already carries `type uint16`, `range "0..65535"`, `units "seconds"` and `default 0`, which is the maximum native validation for an interval |
| YANG custom validators | N-A | The native range is sufficient; no cross-leaf constraint applies |
| CLI commands/flags | No | No command is added. `ze show bmp collectors` is unchanged |
| CLI grammar (keyword before value) | N-A | No command surface changed |
| Editor autocomplete | No | Automatic for the existing typed leaf |
| Functional test for new RPC/API | Yes | `test/plugin/bmp-sender-statistics.ci` |
| Pipe completeness | N-A | No command output changed |
| Env var registration | N-A | The leaf is config, not an environment leaf |
| Doctor check for runtime dependencies | No | No new file, socket, port, module or binary. The reports travel on the collector connection the sender already opens |
| Prometheus counters/metrics | No | The duplicate count is a BMP wire counter, per peer and per BMP session, and resets with the peer. Publishing it as a metric is a different lifetime and is not part of this spec |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute is touched |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/bmp.md`; `docs/features.md` names BMP as a whole and lists no per-leaf behavior, so it is unaffected |
| 2 | Config syntax changed? | No | The leaf and its syntax are unchanged; only what ze does with it changed |
| 3 | CLI command added/changed? | No | No command surface changed |
| 4 | API/RPC added/changed? | No | No RPC changed |
| 5 | Plugin added/changed? | No | No plugin is added, moved or removed |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md`: the leaf table, the message-type table, the sender behavior list and the bounce section |
| 7 | Wire format changed? | No | The Statistics Report encoding is unchanged; it now has a caller |
| 8 | Plugin SDK/protocol changed? | No | No SDK or process-protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8671.md` (RFC8671-6.2-1 is no longer a gap) and `rfc/extraction/rfc8671.json` site 6.2:1. `rfc/short/rfc7854.md` needed no edit: RFC7854-x-15 was already declared and now carries both polarities. `docs/features/rfc-status.md` is generated by `./le rfc index-update`, which was run |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` is unaffected: the `statistics` collector mode is one more mode of the existing `bmpCollector04` fixture and the interop scenario follows the naming and discovery rules the page already states |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` lists BMP support, not per-message behavior |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` describes the plugin lifecycle, which is unchanged. `docs/architecture/testing/interop.md` is declared by `internal/le/interoplab/bgp/names.go` and is unaffected: the new scenario follows its rules rather than changing them |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No metric added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers differently |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED by `./le spec citation anchors`: it named `docs/architecture/testing/interop.md`, declared by `internal/le/interoplab/bgp/names.go`, which row 12 answers as unaffected |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/bmp.md` shows a sender block with `statistics-timeout 0`; the example is still valid and its description row was corrected |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the leaf reaches a timer over the engine's own config rail
   - Tests: `TestRFC7854StatisticsTimeoutSendsPeriodicReports`, `TestRFC7854StatisticsTimeoutZeroSendsNoReport`
   - Files: `statistics.go` (`setStatisticsTimeout`, `statisticsLoop`), `sender_config.go` (`applySenderConfig`), `bmp.go` (the two fields)
   - Verify: with the install call cut, the wiring test times out waiting for reports (observed, and recorded in `rfc/discrimination/rfc7854.json`)
2. **Phase: the counter** -- one statistic ze measures, and measures honestly
   - Tests: `TestBMPDuplicateUpdateCountsReceivedRepeatsOnly`, `TestBMPStatisticsMeasuresTheDirectionThePolicyAlsoStreams`, `TestBMPStatisticsMeasuresUnderAPolicyThatStreamsNothing`, `TestBMPStatisticsOffMeasuresNothingUnderAPolicyThatStreamsNothing`
   - Files: `bmp.go` (`dedupSentSalt`, `dedupKey`, `dedupCount`), `bmp_events.go` (`policyStreams`, `duplicateUpdate`, `handleSenderUpdate`)
   - Verify: a repeat in the received direction counts, a repeat in the sent direction does not, and the detector runs under a policy that streams neither
3. **Phase: the message** -- one report per peer, carrying that counter
   - Tests: `TestBMPStatisticsReportCarriesTheDuplicateCounter`, `TestRFC8671StatisticsReportOnTheWireClearsTheOFlag`
   - Files: `statistics.go` (`sendStatisticsReports`, `makeStatCounter`)
   - Verify: the report carries Stat Type 13 as 4 bytes, names its peer, and clears the O flag while leaving the L flag
4. **Phase: the operator path** -- a running daemon, a socket, and another implementation
   - Tests: `test/plugin/bmp-sender-statistics.ci`, interop scenario `bmp-statistics-pmacct`
   - Files: `internal/test/fixture/plugin_fixture_04_bmp.go`, `internal/le/interoplab/bgp/*`, `test/interop/scenarios/bmp-statistics-pmacct/*`
   - Verify: pmacct decodes the reports; with the install call cut the scenario goes RED (observed 2026-09-06)
5. **Phase: the record** -- every page and ledger the change made wrong
   - Files: `docs/guide/bmp.md`, `rfc/short/rfc8671.md`, `rfc/extraction/rfc8671.json`, `plan/journal/unwired-feature.md`, and the generated `ai/RFC-REQUIREMENTS.md`, `rfc/requirements/`, `docs/features/rfc-status.md`
   - Verify: `./le rfc check` names no rfc7854 or rfc8671 finding

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 `statistics.go::statisticsLoop`; AC-2 `statistics.go::setStatisticsTimeout`; AC-3 `statistics.go::sendStatisticsReports` and `sender.go::writeStatisticsReport`; AC-4 `bmp_events.go::duplicateUpdate`; AC-5 `statistics.go::setStatisticsTimeout`; AC-6 `bmp_events.go::handleSenderUpdate`; AC-7 `sender_config.go::applySenderConfig` |
| Feature completeness | Each of the three user stories has a passing test, and story 2 is a third-party decode rather than ze reading its own bytes |
| Correctness | A report carries only a stat type ze measures; the counter counts the RECEIVED direction only; a zero is a measured zero rather than an absent measurement |
| Naming | The stat type constant names the RFC's own type, and `statTypeDuplicateUpdates` is the only one declared, because ze measures no other |
| Data flow | The timer is installed in `applySenderConfig` and nowhere else; `setStatisticsTimeout` is the only place a ticker starts or stops |
| Rule: `ai/rules/principles.md` | No silently-wrong value: a peer with no dedup entry reports a zero the detector did take, which is why the detector runs for a direction the policy filters out |
| Rule: `ai/rules/goroutine-lifecycle.md` | The ticker goroutine has two exits, both closed channels, and joins `bp.sessions`, which `runBMPPlugin` waits on after closing `stopCh` |
| Rule: `ai/rules/rfc-compliance.md` | RFC8671-6.2-1 moved from a gap to proven, and the ledger, the extraction site and the encoder test doc comment were all corrected with the code |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The encoder has a non-test caller | `gopls references internal/component/bgp/plugins/bmp/sender.go:718:26` names `statistics.go` |
| The unit and tagged tests pass under the race detector | `./le job run label unit-bmp command go test -race ./internal/component/bgp/plugins/bmp/` |
| Another implementation decodes the reports | `INTEROP_SCENARIO=bmp-statistics-pmacct ./le integration interop` |
| The tags carry recorded reds | `./le rfc discriminate stem rfc7854` and `./le rfc discriminate stem rfc8671` |
| No RFC finding names 7854 or 8671 | `./le rfc check` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | `statistics-timeout` is operator config, already bounded by the YANG range. A value the tree cannot deliver as a number installs no report rather than a default one, so a garbled config cannot start a report stream nobody asked for |
| Resource exhaustion | One goroutine per configuration, not per peer or per collector. The per-peer hash set stays capped at `maxDedupPerPeer`, and the counter is one `uint32` per peer. A collector that stops reading has its session reset by the existing transmit-queue policy rather than growing a queue |
| Information disclosure | A report carries a per-peer header and one counter, all of which the collector already receives on Peer Up and Route Monitoring |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- **"At least one statistic MUST be carried" is what decides the design, not the timer.** RFC 7854 Section 4.8 defines fourteen stat types and forbids an empty report. The timer was an hour's work; the question that took the thinking was which of the fourteen ze can report without inventing a number. Thirteen of them count things the reactor holds and the BMP plugin does not. The fourteenth, Stat Type 13, counts something the plugin already computes for another reason.
- **A counter forces a correctness question the feature it feeds never asked.** Route Monitoring dedup only has to answer "have I seen this body", and one hash space per peer answers it. "Number of duplicate update messages RECEIVED" cannot be answered from that space, because a body ze advertised is in it too. The counter is what made the shared space visibly wrong.
- **A gate can decide a data shape.** The obvious fix is a per-peer struct holding both hash sets and the counter, and it was written. It changed one line inside an RFC-tagged test, and `./le commit create` refuses that without an owner-approved row in `test/rfc-changed/`. Salting the sent direction into the same map reaches the same correctness with the committed type intact, so the tagged test never moves. The lesson is not "avoid the gate": it is that a type on a struct a tagged test constructs is part of that test's surface.
- **A test that measures nothing must not report a zero.** With `route-monitoring-policy post-policy` the received direction reaches no collector, so the obvious implementation skips it entirely and the report carries 0. That zero reads to a collector as "no duplicates", which is a different claim from "not measured". `handleSenderUpdate` therefore runs the detector for a direction it will not stream, whenever an interval is configured.
- **A plugin-lifetime goroutine on a shared WaitGroup makes a test helper's contract load-bearing.** `liveCollectorSession` waited on `bp.sessions` without closing `bp.stopCh`, which was harmless while only collector sessions joined that group. The ticker joined it and the helper hung for ever. `BMPPlugin`'s own doc comment already said "Caller MUST close stopCh"; the helper was the caller that did not.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Report Stat Type 13 only | Report types 7 and 8 as gauges from the RIB plugin; report a zero for several types; send an empty report | Section 4.8 forbids an empty report, and a type ze does not measure would publish a zero a collector reads as a real count. The RIB counts live in another plugin and reaching for them is a second feature |
| One ticker per configuration, in the plugin | One ticker per collector session; one per peer | The report set is decided by configuration, not by a session: a per-session ticker would send two collectors reports at different phases for no reason, and a per-peer ticker is one goroutine per peer for a message that is per peer anyway |
| Salt the sent direction into one per-peer hash set | Two sets in a per-peer struct; one shared set and count only received events | A shared space makes a body ze received and later advertised a "duplicate" in the sent direction, so ze suppresses a Route Monitoring it owes and, on the reverse order, counts a receive that never happened. The struct was written and reverted: it changed an RFC-tagged test, which needs owner approval this session does not have. The salt keeps one entry per peer, so there is still one lookup, one memory cap and one `delete` |
| Measure the received direction whatever the policy | Measure only what is streamed | A counter that stops being measured under `post-policy` publishes an untaken zero (`ai/rules/principles.md`) |
| Reset the counter when the peer is bounced | Keep it across a Peer Down/Peer Up pair | A Peer Down withdraws the peer's routes at the collector, so the peer that comes back is a new peer to it. `bounceMonitoredPeers` already clears the dedup state for that reason |
| A new interop scenario against pmacct | Add the assertion to `bmp-frr` | `bmp-frr` reads ze's stream with ze's own collector. Ze agreeing with itself is not interop evidence |

## Known Limitations
- Only Stat Type 13 is reported. The other thirteen types are not a deferral of this spec: each needs a counter ze does not hold, and types 7 to 10 need the RIB plugin to publish route counts to the BMP plugin, which is a feature of its own.
- The counter is a floor rather than an exact total for a peer that churns more than `maxDedupPerPeer` (100,000) distinct UPDATE bodies: past the cap a repeated body is neither suppressed nor counted. That cap is pre-existing and bounds the memory the dedup state can take.
- The RFC 9069 Loc-RIB emulated peer gets no Statistics Report. RFC 9069 Section 5.6 names types 8 and 10 as the relevant ones and both are Loc-RIB route counts the BMP plugin does not hold.

## RFC Documentation (Scope: protocol)

Every enforcing site carries its RFC citation in the code:

| Code | RFC text quoted above it |
|------|--------------------------|
| `statistics.go::statTypeDuplicateUpdates` | RFC 7854 Section 4.8: "Stat Type = 13: (32-bit Counter) Number of duplicate update messages received." |
| `statistics.go::statCounterSize` | RFC 7854 Section 4.8: "Although the current specification only specifies 4-byte counters and 8-byte gauges as "Stat Data"..." |
| `statistics.go::setStatisticsTimeout` | RFC 7854 Section 4.8: "SR messages are optional." -- which is what a zero interval takes |
| `statistics.go::sendStatisticsReports` | RFC 7854 Section 4.8 on timing ("It is left to the implementation to determine transmission timings -- however, configuration control should be provided of the timer and/or threshold values"), on the one-statistic minimum, and RFC 9069 Section 5.6 on why the Loc-RIB peer gets none |
| `bmp.go::dedupCount` | RFC 7854 Section 4.8 on what a 32-bit Counter is, which is why the count is a `uint32` |
| `bmp_events.go::duplicateUpdate` | RFC 7854 Section 4.8 Stat Type 13, and why only the received direction counts |
| `sender.go::writeStatisticsReport` | RFC 8671 Section 6.2: "Statistics report messages are not specific to Adj-RIB-In or Adj-RIB-Out and MUST have the O flag set to zero." (already present) |
| `internal/test/fixture/plugin_fixture_04_bmp.go::validateStatistics04` | RFC 7854 Section 4.8 on the body layout and the one-statistic minimum, and RFC 8671 Section 6.2 on the O flag |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
