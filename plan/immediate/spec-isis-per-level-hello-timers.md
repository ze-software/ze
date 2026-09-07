# Spec: isis-per-level-hello-timers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.** Four leaves of
`internal/plugins/isis/yang/ze-isis-conf.yang` fail for one reason and share this
spec: `level-1/hello-interval`, `level-1/hold-multiplier`,
`level-2/hello-interval` and `level-2/hold-multiplier`, all under
`interfaces/interface`. Their descriptions are "L1 hello-interval override.", "L1
hold-multiplier override." and the Level-2 pair of the same. The enclosing
containers are described "Level-1 per-interface overrides." and "Level-2
per-interface overrides." A reader takes each leaf to replace the circuit-wide
value at one level, which is exactly what the sibling `metric` and `priority`
leaves of the same containers do. Under that reading a circuit can run a fast
Level-1 hello and a slow Level-2 hello, and can advertise a different holding
time at each level.

**What Ze did instead, before this spec.** `parseLevelInterface`
(`internal/plugins/isis/config.go`) stored the four values in
`LevelInterfaceConfig.HelloInterval` and `.HoldMult`. Neither field had a reader.
The circuit ran one hello timer for both levels: `launchCircuitGoroutine`
(`internal/plugins/isis/circuits.go`) built a single ticker from the circuit-wide
`ic.HelloInterval`, and every tick called `SendHello`
(`internal/plugins/isis/circuit/runtime.go`), which sent one IIH per configured
level through `sendLANHellos`. The holding time came from the same place:
`buildCircuit` passed `ic.HelloInterval` and `ic.HoldMult` into `circuit.New`,
which precomputed one `HoldTime(cfg.HelloInterval, cfg.HoldMult)`. The four
leaves changed nothing, and the committed `ze:help` on both containers said so in
the sentence "`hello-interval` and `hold-multiplier` are stored and not acted
on", while the four `description` strings still said override.

**What closing it means.** The implementer either builds the per-level timers or
refuses the four leaves at commit, the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses the `vrf` leaf. This spec
BUILDS them. The deciding fact is the circuit kind, and the design answers it:
a broadcast circuit runs one timer per level, and a point-to-point circuit runs
one timer for the single level-agnostic IIH it sends.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/isis/isis-5-adjacency.md` - the page that owns IIH origination, padding and hold time
  → Decision: the circuit performs I/O only through the injected `Sender`, so the timer decision belongs to the circuit and the ticker to the engine
  → Constraint: padding is added before authentication and the transport adds only framing, so a per-level send path must keep the build/pad/sign order
- [ ] `docs/architecture/testing/interop.md` - the suite an interop scenario is owed against
  → Constraint: a scenario directory is NAMED, carries only declarative inputs, and its assertions are a typed Go checker under `internal/le/interoplab/`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5303.md` - the point-to-point IIH and its three-way TLV
  → Constraint: RFC 5303 defines TLV 240 and nothing about levels. `grep -n -i level rfc/full/rfc5303.txt` answers one line, the RFC 2119 boilerplate, so it is NOT authority for the level-agnostic point-to-point IIH and the closure moved every such citation off it
- [ ] `rfc/full/rfc1195.txt` section 5.3 - the PDU list Ze cites for the level-agnostic point-to-point IIH
  → Constraint: 5.3 names the LAN IIH once per level (5.3.1 "Level 1 LAN IS to IS Hello PDU", 5.3.2 "Level 2 LAN IS to IS Hello PDU") and the point-to-point IIH once with no level in its name (5.3.3 "Point-to-Point IS to IS Hello PDU"), so a point-to-point circuit has one IIH to time and one holding time to advertise
- [ ] `iso/short/iso10589.md` - the hold time definition Ze implements
  → Constraint: hold time = hello interval * hold multiplier. The summary says nothing about a per-level timer, and the ISO text is not in the repository, so the committed `ze:help` citation of "clause 10.9" was NOT verifiable and was removed rather than repeated

**Key insights:** (minimal context to resume after compaction)
- The Level-1 and Level-2 LAN IIH are separate PDU types (15 and 16) sent to separate multicast groups (`internal/plugins/isis/transport/multicast.go`), which is what makes two periods meaningful on a LAN and meaningless on a point-to-point link.
- `levelMetric` (`lsdb_wiring.go`) and `disPriority` (`dis_wiring.go`) already resolve the sibling per-level overrides. `levelHelloTimers` mirrors them, so the block is now uniformly wired.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/isis/config.go` - `parseInterface` applies the YANG defaults; `parseLevelInterface` stores the per-level container with no defaults, a zero field meaning "inherit"
- [ ] `internal/plugins/isis/circuits.go` - `buildCircuit` fills `circuit.Config`; `launchCircuitGoroutine` runs the hello and sweep tickers
- [ ] `internal/plugins/isis/circuit/circuit.go` - `New` freezes the config into the circuit; the circuit held one `holdTime`
- [ ] `internal/plugins/isis/circuit/runtime.go` - `SendHello`, `sendLANHellos`, `sendP2PHello`, `p2pPreferredLevel`, `formsLevel`
- [ ] `internal/plugins/isis/circuit/hello.go` - `HoldTime` and the two IIH builders that set the Holding Time field
- [ ] `internal/plugins/isis/server.go` - `circuitParamsEqual`, the reconcile comparison over the fields that affect a running circuit

**Behavior to preserve:** (unless the user explicitly said to change it)
- The circuit-wide `hello-interval` and `hold-multiplier` still govern a level with no override, and the YANG defaults (10 and 3) still govern a circuit with neither.
- The IIH build order stays origination TLVs, then TLV 8 padding, then signing, then framing.
- A point-to-point circuit still sends ONE IIH to both multicast groups, signed at the negotiated adjacency level.

**Behavior to change:** (only what the user asked for)
- A broadcast circuit sends the Level-1 and the Level-2 IIH at periods of their own.
- Each IIH advertises the holding time of its own level.
- `SendHello` takes the level it is sending for, and refuses a level the circuit does not form.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The `isis` config subtree, from a config file or a `ze config` commit: `interfaces/interface/<name>/level-1/hello-interval` and `hold-multiplier`, and their level-2 twins.
- Format at entry: string-typed YANG leaves inside a keyed list, as `map[string]any`.

### Transformation Path
1. `parseInterface` / `parseLevelInterface` (`internal/plugins/isis/config.go`) resolve the subtree into `InterfaceConfig` and `LevelInterfaceConfig`.
2. `levelHelloTimers` (`internal/plugins/isis/circuits.go`) merges the per-level override over the circuit-wide leaf over the YANG default, once per level.
3. `buildCircuit` puts the two resolved pairs in `circuit.Config.Level1` and `.Level2`; `circuit.New` freezes them.
4. `HelloSchedules` (`internal/plugins/isis/circuit/runtime.go`) answers one schedule per level on a broadcast circuit and exactly one on a point-to-point circuit.
5. `launchCircuitGoroutine` starts one ticker per schedule; each tick calls `SendHello(level)`.
6. `buildLANHello` / `buildP2PHello` (`circuit/hello.go`) set the Holding Time field from `c.holdTime(level)`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → engine | `parseISISConfig` into typed `InterfaceConfig` | Yes -- `TestISISLevelHelloTimersResolution` starts from the config text |
| Engine → circuit | `circuit.Config.Level1` / `.Level2` | Yes -- `TestISISPerLevelHelloTimersReachTheWire` builds the circuit through `buildCircuit` |
| Circuit → transport | `SendPDU(name, level, pdu)` to the level's multicast group | Yes -- the same test asserts the destination group per level |
| Ze → FRR | the Holding Time field of each LAN IIH | Yes -- `isis-per-level-hello-frr` reads both numbers back out of FRR |

### Integration Points
- `levelMetric` and `disPriority` - `levelHelloTimers` mirrors their override-then-circuit-wide shape, so all four per-level kinds now resolve the same way.
- `circuitParamsEqual` (`server.go`) - the per-level containers now select what the circuit sends, so the reconcile comparison includes them.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The circuit never reads config: `levelHelloTimers` resolves and `circuit.New` freezes |
| No unintended coupling (components stay isolated) | Yes | `circuit` gains a `LevelTimers` value type and learns nothing about YANG |
| No duplicated functionality (extends existing, does not recreate) | Yes | One resolver beside `levelMetric` and `disPriority`; `HoldTime` is unchanged and still the only hold-time arithmetic |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No new allocation on the send path: `HelloSchedules` is built once at circuit start, and `sendLANHello` builds one PDU where `sendLANHellos` built one per level |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Everything lands inside the `isis` plugin and its `circuit` package; no core or shared package names the feature |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A broadcast circuit's two levels are separate PDUs to separate groups, so two periods are meaningful | `internal/plugins/isis/transport/multicast.go`, `packet/header.go` PDU types 15 and 16 | Two tickers would send the same PDU twice | `TestISISPerLevelHelloTimersReachTheWire` asserts the destination group per level | confirmed |
| A-2 | A point-to-point IIH carries no level, so it can only be timed once | `rfc/full/rfc1195.txt` section 5.3 (5.3.1/5.3.2 name a LAN IIH per level, 5.3.3 names one point-to-point IIH), `sendP2PHello` sends to both groups | Two P2P tickers would double the IIH rate | `TestISISP2PRunsOneHelloSchedule` | confirmed; its basis was corrected at closure from `rfc/short/rfc5303.md`, which says nothing about levels |
| A-3 | FRR records the holding time of each level's IIH separately | FRR's `show isis neighbor` prints one row per level with a Holdtime column | The interop assertion would read one number for two levels | the red run read 28 twice and the green run read the two overrides apart | confirmed |
| A-4 | No committed config can ask for a zero per-level timer, so a zero means "not set" | `ze-isis-conf.yang` bounds both leaves (1..65535, 1..255) | A legitimate zero would be read as unset | `TestISISLevelHelloTimersResolution` covers the unset, half-set and maximum cases | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A zero period reaches `time.NewTicker` and panics the daemon | a Go caller builds a `circuit.Config` with no timers | `helloPeriod` floors the period at one second, covered by `TestISISHelloScheduleNeverZeroPeriod` |
| R-2 | A caller asks `SendHello` for a level the circuit does not form and an unconfigured level reaches the wire | none, a silent send leaves no trace | `SendHello` returns an error, covered by `TestISISSendHelloRefusesUnformedLevel` |
| R-3 | A committed change to a per-level timer is invisible to a running circuit | the operator commits and nothing changes | `circuitParamsEqual` now compares the containers. Applying a timer change to a LIVE circuit is a pre-existing gap that covers the circuit-wide leaf identically: see Known Limitations |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every IS-IS adjacency: a wrong period or holding time flaps or drops adjacencies on every circuit, not only on a circuit that sets an override |
| How is it reverted? | A single commit revert. Nothing is persisted and no config migration is involved |
| Who else touches this path? | `internal/plugins/isis/` is one plugin; the interop checker registry in `internal/le/interoplab/bgp/` is shared with the BGP and OSPF scenarios and is only added to |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `interfaces/interface/eth0/level-1/hello-interval` in the config text | → | `levelHelloTimers` → `buildCircuit` → `HelloSchedules` → `SendHello` → the Holding Time field | `TestISISPerLevelHelloTimersReachTheWire` (`internal/plugins/isis/level_hello_timers_test.go`) |
| the same config in a running daemon on a LAN with FRR | → | the per-level tickers and the per-level holding time on the wire | `isis-per-level-hello-frr` (`checkISISPerLevelHelloTimers`, `internal/le/interoplab/bgp/check_isis.go`) |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `level-1 { hello-interval 3 }` on a circuit whose circuit-wide `hello-interval` is 10 | Level-1 resolves to 3 and Level-2 keeps 10 |
| AC-2 | A broadcast circuit forming both levels, each with its own `hello-interval` | The circuit runs one hello timer per level, each at that level's period |
| AC-3 | The same circuit sending an IIH at each level | Each IIH carries the holding time of its own level, and goes to that level's multicast group |
| AC-4 | A point-to-point circuit forming both levels | The circuit runs ONE hello timer, and the single IIH advertises the holding time of the level that timer runs at |
| AC-5 | `SendHello` called for a level the circuit does not form | It returns an error and sends nothing |
| AC-6 | A level with no override, or a circuit with no leaf set anywhere | The circuit-wide value governs, and the YANG default governs when there is none |
| AC-7 | A `circuit.Config` that resolved no timers | The published period is positive, so the engine's ticker cannot panic |
| AC-8 | A reload whose only edit is a per-level hello timer | The reconcile reports the circuit changed rather than unchanged |
| AC-9 | An FRR peer on a LAN with Ze, Ze setting a different pair at each level | FRR reads back one holding time per level, each the one its own level asked for |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs a fast Level-1 hello and a slow Level-2 hello on one LAN circuit | config text → `levelHelloTimers` → `circuit.Config` → `HelloSchedules` → two tickers → two IIHs | `TestISISPerLevelHelloTimersReachTheWire`, `isis-per-level-hello-frr` |
| 2 | advertises a longer holding time at Level-2 than at Level-1 on one circuit | the same path → `buildLANHello` → the Holding Time field | `TestISISPerLevelHoldingTimeInLANIIH`, `isis-per-level-hello-frr` assertion 2 |
| 3 | sets a per-level timer on a point-to-point circuit | the same path → one schedule → `buildP2PHello` | `TestISISP2PRunsOneHelloSchedule`, `TestISISP2PL2OnlyTakesItsOwnTimers` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestISISLevelHelloTimersResolution` | `internal/plugins/isis/level_hello_timers_test.go` | AC-1, AC-6: the override, half-override, unset and maximum cases, each from the config text | pass |
| `TestISISPerLevelHelloSchedulesOnBroadcast` | `internal/plugins/isis/circuit/hello_level_timers_test.go` | AC-2: one schedule per level at its own period | pass |
| `TestISISPerLevelHelloScheduleSingleLevel` | `internal/plugins/isis/circuit/hello_level_timers_test.go` | AC-2: a one-level circuit takes its own level's period | pass |
| `TestISISPerLevelHoldingTimeInLANIIH` | `internal/plugins/isis/circuit/hello_level_timers_test.go` | AC-3: each LAN IIH carries its own level's holding time | pass |
| `TestISISSendHelloRefusesUnformedLevel` | `internal/plugins/isis/circuit/hello_level_timers_test.go` | AC-5: the refusal, and that nothing is sent | pass |
| `TestISISP2PRunsOneHelloSchedule` | `internal/plugins/isis/circuit/hello_level_timers_test.go` | AC-4: one schedule and a matching holding time | pass |
| `TestISISP2PL2OnlyTakesItsOwnTimers` | `internal/plugins/isis/circuit/hello_level_timers_test.go` | AC-4: an L2-only P2P circuit takes the Level-2 pair | pass |
| `TestISISHelloScheduleNeverZeroPeriod` | `internal/plugins/isis/circuit/hello_level_timers_test.go` | AC-7: the period floor | pass |
| `TestISISLevelTimerChangeIsNotUnchanged` | `internal/plugins/isis/level_hello_timers_test.go` | AC-8: reconcile sees the change | pass |
| `TestISISPerLevelHelloNeighborTable` | `internal/le/interoplab/bgp/bgp_test.go` | the column reading the interop checker depends on | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `level-N/hello-interval` | 1-65535 seconds | 65535 (resolved unchanged) | 0 is refused by the YANG range and read as "unset" by the resolver | 65536 refused by the YANG range |
| `level-N/hold-multiplier` | 1-255 | 255 (resolved unchanged) | 0 is refused by the YANG range and read as "unset" by the resolver | 256 refused by the YANG range |
| resolved holding time | 1-65535 seconds | 65535 | `HoldTime` clamps a zero product up to 1 | `HoldTime` clamps an overflowing product down to 65535 |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-config` | `test/isis/isis-config.ci` | the per-interface leaves validate through the real YANG schema | pass, unchanged: the level containers were already accepted, so a `.ci` cannot tell the override from the gap. `TestISISPerLevelHelloTimersReachTheWire` runs the whole config-to-wire path in one process, and `isis-per-level-hello-frr` runs it in a daemon |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `isis-per-level-hello-frr` | `test/interop/scenarios/` | FRR 10.3.1 | AC-9: FRR decodes the Holding Time of each level's LAN IIH and reads back 9 seconds at Level-1 and 120 at Level-2, on a circuit whose circuit-wide pair of 10 and 3 advertises 30 at both levels. The Level-1 adjacency also survives a window longer than the 9 seconds it advertised, which only a 3-second Level-1 period sustains | pass; RED proven by reverting `levelHelloTimers` |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/plugins/isis/circuit/circuit.go` - `LevelTimers`, the per-level `Config` fields, the stored pairs, `timers` and `holdTime(level)`
- `internal/plugins/isis/circuit/runtime.go` - `HelloSchedule`, `HelloSchedules`, `helloPeriod`, `SendHello(level)` and its refusal, `sendLANHello`
- `internal/plugins/isis/circuit/hello.go` - both IIH builders take the holding time of a level
- `internal/plugins/isis/circuits.go` - `levelHelloTimers`, the per-level `circuit.Config` fields, one ticker per schedule
- `internal/plugins/isis/server.go` - `circuitParamsEqual` compares the per-level containers
- `internal/plugins/isis/yang/ze-isis-conf.yang` - the six help strings that said the leaves were not acted on
- `docs/architecture/isis/isis-5-adjacency.md` - the page `circuit/hello.go` and `circuit/runtime.go` declare
- `docs/architecture/isis/isis-4-component-config.md` - the page `server.go` declares. Named as UNAFFECTED: its one reconcile sentence says the journal diff "flaps no circuit" on a metric-only change, which is still true with the per-level containers in the comparison, and the page describes no field list
- `internal/le/interoplab/bgp/check_isis.go`, `check_special.go` - the interop checker and its registration

## Files to Create
- `internal/plugins/isis/circuit/hello_level_timers_test.go` - the per-level schedule and holding-time unit tests
- `internal/plugins/isis/level_hello_timers_test.go` - the resolver table and the config-to-wire wiring test
- `test/interop/scenarios/isis-per-level-hello-frr/ze.conf`, `frr.conf` - the interop scenario inputs

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The four leaves already exist; only their help text changed |
| YANG validation constraints | No | `range "1..65535"` and `range "1..255"` were already declared and are unchanged |
| YANG custom validators | No | The native ranges are sufficient; a zero is out of range, so no validator is needed to read one as unset |
| CLI commands/flags | No | No command is added or changed |
| CLI grammar (keyword before value) | N-A | No command surface in this change |
| Editor autocomplete | No | Both leaves are typed `uint16`/`uint8`, so completion is already automatic |
| Functional test for new RPC/API | N-A | No RPC or API is added |
| Pipe completeness | N-A | No new output |
| Env var registration | N-A | No `environment/` leaf |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module or binary; the circuit's raw socket is unchanged |
| Prometheus counters/metrics | No | The existing `ze_isis_adjacencies_up` and frame counters cover the send path unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | IS-IS |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The leaves were already published; this makes them true. `docs/features.md` names no timer leaf |
| 2 | Config syntax changed? | No | No leaf added, moved or renamed |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | Behavior inside an existing plugin |
| 6 | Has a user guide page? | No | `docs/guide/isis.md` documents hostname, DIS, authentication, redistribution, dual-stack and observation, and carries no timer section. The operator-facing text for these leaves is their `ze:help`, which is corrected here |
| 7 | Wire format changed? | No | The Holding Time field and both IIH layouts are unchanged; only the value Ze puts in the field |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC requirement id is added or re-levelled. The behavior is ISO/IEC 10589, whose text is not in the repository, and `rfc/short/` holds no requirement for it |
| 10 | Test infrastructure changed? | Yes | A named interop scenario and its typed checker. `docs/architecture/testing/interop.md` describes the suites and the discovery, and lists no individual scenario, so it stays correct and needs no edit |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares protocol support, not timer granularity |
| 12 | Internal architecture changed? | Yes | `docs/architecture/isis/isis-5-adjacency.md` gains "Decision: one hello timer per level, not per circuit" |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `ai/CODE-TO-DOCS.md` maps `circuit/hello.go` and `circuit/runtime.go` to `docs/architecture/isis/isis-5-adjacency.md`, which is updated. `server.go` declares `docs/architecture/isis/isis-4-component-config.md`, named as unaffected under Files to Modify with the reason. No other changed file is named by an anchor |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The IS-IS interop scenarios show `hello-interval` directly under `interface`, which stays valid and is still the circuit-wide leaf |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- the config value reaches the wire
   - Tests: `TestISISPerLevelHelloTimersReachTheWire`, `TestISISLevelHelloTimersResolution`
   - Files: `internal/plugins/isis/circuits.go` (`levelHelloTimers`, `buildCircuit`), `internal/plugins/isis/circuit/circuit.go` (`LevelTimers`)
   - Verify: with the resolver ignoring the override, the test reports the circuit-wide value at both levels
2. **Phase: Per-level send** -- one timer and one holding time per level
   - Tests: the `hello_level_timers_test.go` set
   - Files: `circuit/runtime.go` (`HelloSchedules`, `SendHello`), `circuit/hello.go`, `circuits.go` (the tickers)
   - Verify: with `timers` answering Level-1 for every level, the schedule and holding-time tests go red
3. **Phase: Reconcile and schema truth** -- the operator's committed change is seen, and the help stops contradicting the code
   - Tests: `TestISISLevelTimerChangeIsNotUnchanged`
   - Files: `internal/plugins/isis/server.go`, `internal/plugins/isis/yang/ze-isis-conf.yang`
   - Verify: dropping the two comparisons reddens the reconcile test
4. **Phase: Interop** -- an independent implementation reads both holding times
   - Tests: `isis-per-level-hello-frr`
   - Files: the scenario directory, `internal/le/interoplab/bgp/check_isis.go`, `check_special.go`, `bgp_test.go`
   - Verify: reverting `levelHelloTimers` and rebuilding the image turns the scenario red on assertion 2

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The merge order is override, then circuit-wide, then YANG default, and it is applied per level rather than per circuit |
| Naming | `LevelTimers` and `HelloSchedule` name what they carry; the YANG leaf names are unchanged |
| Data flow | The circuit reads no config: resolution happens once in `levelHelloTimers` and the circuit holds frozen values |
| Rule: `ai/rules/principles.md` | `SendHello` refuses a level it does not form rather than returning silently, and `helloPeriod` never answers zero |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| The four leaves change what Ze sends | `go test ./internal/plugins/isis/...` |
| An independent daemon reads both holding times | `INTEROP_SCENARIO=isis-per-level-hello-frr ./le integration interop` |
| The schema no longer says the leaves are inert | `grep -c "stored and not acted on" internal/plugins/isis/yang/ze-isis-conf.yang` answers 0 |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | Both leaves are bounded by the YANG range before the resolver sees them, and the resolver reads a zero as unset rather than as a value |
| Resource exhaustion | The smallest period an operator can ask for is 1 second, unchanged from the circuit-wide leaf, and a circuit runs at most two tickers because it forms at most two levels |
| Error leakage | The `SendHello` refusal names the circuit and the level, and carries no key material or peer data |

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
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- The circuit kind is what makes the leaves meaningful, and it decides the SHAPE of the answer rather than whether to answer. A broadcast circuit gets one schedule per level because the two IIHs are separate PDUs; a point-to-point circuit gets one because its IIH is level-agnostic. Neither case needs a refusal, so the "refuse at commit" arm of the Task section was not taken.
- Publishing a `[]HelloSchedule` rather than exposing the timers keeps the ticker decision in the engine and the level decision in the circuit. It is also what makes the point-to-point case a one-element slice instead of a special case in the engine loop.
- The advertised holding time must be derived from the period the IIH REALLY goes out at, not from the level a signature is computed for. On a point-to-point circuit those differ: the signature follows the negotiated adjacency level and the holding time follows the schedule.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build the per-level timers | Refuse the four leaves at commit, like `unimplementedVRFValidator` | The behavior is legible and the sibling `metric` and `priority` overrides already work this way. A refusal would have removed a promise the operator can reasonably expect and left the block half-wired |
| A point-to-point circuit runs ONE timer, at its preferred level | Two timers; the shortest of the two periods; refusing the leaves on a point-to-point circuit | Its IIH carries no level, so two timers would double the IIH rate for nothing. The preferred level is what `p2pPreferredLevel` already picks for signing an IIH with no negotiated adjacency, so the circuit answers the level question once |
| `SendHello` takes a level and refuses one the circuit does not form | Keep `SendHello()` sending every level | The engine now owns one ticker per level, so the level is the caller's fact. Refusing rather than no-opping keeps a caller that reached an unpublished schedule visible |
| The period floor lives in `helloPeriod` | A `clamp` on `LevelTimers` at construction | The schema minimum is declared in the YANG, and the engine applies the default before the circuit is built. Only `time.NewTicker` cannot survive a zero, so the guard sits there and the circuit stores what it was given |
| Compare the whole `LevelInterfaceConfig` in `circuitParamsEqual` | Compare only the two timer fields | The comparison's own comment is "the fields that affect a running circuit", and the container's metric, priority and key chain all do. Comparing the struct keeps it true as the container grows |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- A parameter change to a RUNNING circuit is stored and not applied. `reconcile` (`internal/plugins/isis/server.go`) records the new `InterfaceConfig` and leaves the live circuit alone, and its own comment says runtime application "lands in isis-5/6". This covers the circuit-wide `hello-interval` and `hold-multiplier` identically and predates this spec, so the per-level leaves are not worse off than the leaves they override. A per-level timer set in the config a daemon starts with, or on a circuit that opens after a link-up, is applied in full. Recorded as a journal row rather than fixed here, because rebuilding a live circuit on a parameter change flaps adjacencies and is a decision of its own.
- `show isis interface` reports the circuit-wide `hello-interval` and `hold-multiplier`, as it reports the circuit-wide `metric` beside a live `level-1/metric` override. Making that view per-level is one decision covering all four override kinds, not a timer change.

## RFC Documentation (Scope: protocol)

The behavior is ISO/IEC 10589, not an RFC Ze holds text for. `HoldTime`
(`internal/plugins/isis/circuit/hello.go`) already carries the clause 8.2
citation for hold time = interval * multiplier, and the per-level code repeats
it where it derives a holding time. The one RFC citation this change adds is RFC
1195 section 5.3 for the level-agnostic point-to-point IIH, above
`HelloSchedules` and `buildP2PHello`, which is why a point-to-point circuit runs
one timer. That section names the LAN IIH once per level (5.3.1 "Level 1 LAN IS
to IS Hello PDU", 5.3.2 "Level 2 LAN IS to IS Hello PDU") and the point-to-point
IIH once, with no level in its name (5.3.3 "Point-to-Point IS to IS Hello PDU").

The committed `ze:help` cited "ISO/IEC 10589 section 10.9" for a per-level hello
timer. That citation was NOT verifiable: the standard text is not in the
repository and `iso/short/iso10589.md` is a summary that says nothing about it.
The rewritten help states the mechanism Ze implements and cites nothing it
cannot show.

The implementation wrote "RFC 5303 sec 3" for the level-agnostic point-to-point
IIH, which is the same failure one paragraph up and the closure fixed it. RFC
5303 defines TLV 240 and never mentions a level: `grep -n -i level
rfc/full/rfc5303.txt` answers one line, the RFC 2119 boilerplate in the
references. Four sites in this change and four that predate it were repointed at
RFC 1195 section 5.3, and the class is recorded in
`plan/journal/reference-checked-claim-unchecked.md`.

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

## Red-Then-Green Evidence

Every new test was watched failing against a break of the code it claims to
prove, then passing with the code restored.

| Break applied | Tests that went RED | Observed |
|---------------|--------------------|----------|
| `levelHelloTimers` ignores the per-level override | `TestISISLevelHelloTimersResolution` (5 subtests), `TestISISPerLevelHelloTimersReachTheWire` | `Level-1 timers = {HelloInterval:10 HoldMult:3}, want {HelloInterval:3 HoldMult:6}`; `l1 IIH holding time = 30, want 9`; `l2 IIH holding time = 30, want 60` |
| `circuitParamsEqual` drops the two container comparisons | `TestISISLevelTimerChangeIsNotUnchanged` | `circuitParamsEqual called a level-1 hello-interval change unchanged` |
| `Circuit.timers` answers the Level-1 pair for every level | `TestISISPerLevelHelloSchedulesOnBroadcast`, `TestISISPerLevelHelloScheduleSingleLevel`, `TestISISPerLevelHoldingTimeInLANIIH`, `TestISISP2PL2OnlyTakesItsOwnTimers` | `HelloSchedules()[1] = {Level:l2 Period:3s}, want {Level:l2 Period:30s}`; `L2 IIH hold 9, want 60` |
| `SendHello` drops the `formsLevel` guard | `TestISISSendHelloRefusesUnformedLevel` | `SendHello(Level2) on an L1-only circuit sent 1 PDUs, want 0` |
| `levelHelloTimers` ignores the override, image rebuilt | `isis-per-level-hello-frr` | `FRR read a Level-1 holding time of 28s, over the 15s bound the level-1 override (3 * 3) puts it under`, with FRR's table showing Holdtime 28 at BOTH levels. With the fix restored the scenario passes |

## Implementation Summary

### What Was Implemented
- `levelHelloTimers` (`internal/plugins/isis/circuits.go`) resolves one `circuit.LevelTimers` pair per level: the per-level `hello-interval` / `hold-multiplier` when set, else the circuit-wide leaf, else the YANG default. It mirrors `levelMetric` and `disPriority`, so all four per-level override kinds now resolve the same way.
- `circuit.LevelTimers`, `Config.Level1` and `Config.Level2` (`circuit/circuit.go`) replace the single `Config.HelloInterval` / `HoldMult` pair and the precomputed `holdTime` field. `Circuit.timers(level)` and `Circuit.holdTime(level)` answer per level.
- `HelloSchedules` and `helloPeriod` (`circuit/runtime.go`) publish one schedule per level a broadcast circuit forms and exactly one for a point-to-point circuit. `helloPeriod` floors the period at one second, so a Go-built `Config` with no timers cannot panic `time.NewTicker`.
- `SendHello(level)` (`circuit/runtime.go`) takes the level whose timer fired and returns an error for a level the circuit does not form. `sendLANHellos` became `sendLANHello`, sending one PDU per call.
- `launchCircuitGoroutine` (`circuits.go`) starts one ticker per schedule and sends the initial Hello at every level.
- `circuitParamsEqual` (`internal/plugins/isis/server.go`) compares `Level1` and `Level2` whole, so a committed per-level edit reconciles as a change.
- The six `ze:help` strings on the level containers and the four leaves (`yang/ze-isis-conf.yang`) describe the override the leaves now are.
- `isis-per-level-hello-frr` and `checkISISPerLevelHelloTimers` (`internal/le/interoplab/bgp/check_isis.go`, registered in `check_special.go`) read both holding times back out of FRR 10.3.1.

### Bugs Found/Fixed
- Eight sites cited "RFC 5303 sec 3" for the level-agnostic point-to-point IIH. RFC 5303 defines TLV 240 and never mentions a level. Four sites were this change's, four predate it. All eight now cite RFC 1195 section 5.3. Recorded in `plan/journal/reference-checked-claim-unchecked.md`; no test covers a citation, so nothing was red.
- `docs/functional-tests.md` enumerated "The seven FRR interop scenarios" for IS-IS. Nine exist: this spec added `isis-per-level-hello-frr`, and `isis-max-metric-frr` was already missing. The sentence now names nine and describes the two it omitted.
- The reconcile gap (a parameter change stored and never applied to a running circuit) was found while wiring this and journalled in `plan/journal/unwired-feature.md` with commit `2adbf6a44`. It does not block this spec: see Known Limitations.

### Documentation Updates
- `docs/architecture/isis/isis-5-adjacency.md` gained "Decision: one hello timer per level, not per circuit", with `<!-- source: internal/plugins/isis/circuit/runtime.go -- HelloSchedules, helloPeriod, SendHello -->` and `<!-- source: internal/plugins/isis/circuits.go -- levelHelloTimers, the per-schedule tickers -->`. Its RFC 5303 sentence was repointed at RFC 1195 section 5.3 at closure.
- `docs/architecture/isis/isis-10-auth.md`, "Trap: a point-to-point hello is level-agnostic on the wire": the false claim "RFC 5303 defines one PDU type with no level bit" was replaced with the RFC 1195 section 5.3 evidence. Pre-existing; fixed here because it is the same claim.
- `docs/functional-tests.md`: the IS-IS interop paragraph now names nine scenarios and carries `<!-- source: internal/le/interoplab/bgp/check_isis.go -- checkISISMaxLinkMetric, checkISISPerLevelHelloTimers -->`.
- `./le doc check verify`: see Pre-Commit Verification.
- Verified as needing NO edit: `docs/guide/isis.md` (grep for `hello`/`hold` returns five hits, none a timer section), `docs/architecture/testing/interop.md` (grep for `isis` returns nothing, so it names no scenario), `docs/guide/configuration.md` (the changed strings are `ze:help`, and `internal/le/site/testdata/published-configuration.md` renders `description`, which did not change), `docs/architecture/isis/isis-4-component-config.md` (its one reconcile sentence says a metric-only diff "flaps no circuit", still true).

### Deviations from Plan
- The Task section offered two arms, build the timers or refuse the four leaves at commit. The build arm was taken, as the Key Design Decisions table records. No other deviation.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The change cited RFC 5303 section 3 in four new places for the claim that the point-to-point IIH carries no level | RFC 5303 defines TLV 240 and the word "level" appears once in it, in the RFC 2119 boilerplate. The fact lives in ISO/IEC 10589 clause 9.7, whose text the repository does not hold, and is verifiable in-repo from RFC 1195 section 5.3 | Closure opened `rfc/full/rfc5303.txt` to verify the one RFC citation the spec's RFC Documentation section says it adds | Fixed at all eight sites, four of them pre-existing. Journal row in `plan/journal/reference-checked-claim-unchecked.md` |
| approach | The same spec had just DELETED an unverifiable "ISO/IEC 10589 section 10.9" from the YANG help, and then wrote an unverifiable RFC citation of its own | Removing one bad citation does not check the ones a change adds | The same read | The spec section that ANNOUNCES the citation a change adds is the cheapest place to check it, which is where closure caught this one |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The four per-level leaves change what Ze sends | Done | `levelHelloTimers` (`internal/plugins/isis/circuits.go`) | Was parsed into `LevelInterfaceConfig` with no reader |
| A broadcast circuit runs one hello timer per level | Done | `HelloSchedules` (`circuit/runtime.go`), the two tickers in `launchCircuitGoroutine` (`circuits.go`) | |
| A point-to-point circuit runs one timer for its single level-agnostic IIH | Done | `HelloSchedules`, `p2pPreferredLevel` (`circuit/runtime.go`) | |
| Each IIH advertises the holding time of its own level | Done | `Circuit.holdTime` (`circuit/circuit.go`), `buildLANHello` / `buildP2PHello` (`circuit/hello.go`) | |
| The `ze:help` stops saying the leaves are inert | Done | `yang/ze-isis-conf.yang` | `grep -c "stored and not acted on"` answers 0 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestISISLevelHelloTimersResolution/a_level-1_override_replaces_the_circuit-wide_pair_at_Level-1_only` | Starts from the config text |
| AC-2 | Done | `TestISISPerLevelHelloSchedulesOnBroadcast`, `TestISISPerLevelHelloScheduleSingleLevel` | |
| AC-3 | Done | `TestISISPerLevelHoldingTimeInLANIIH`, `TestISISPerLevelHelloTimersReachTheWire` | The wiring test also asserts the destination multicast group per level |
| AC-4 | Done | `TestISISP2PRunsOneHelloSchedule`, `TestISISP2PL2OnlyTakesItsOwnTimers` | |
| AC-5 | Done | `TestISISSendHelloRefusesUnformedLevel` | Asserts the error AND that nothing was sent |
| AC-6 | Done | `TestISISLevelHelloTimersResolution` cases 1, 2, 5, 6 | Unset, circuit-wide, half-override and default-fallback |
| AC-7 | Done | `TestISISHelloScheduleNeverZeroPeriod` | `helloPeriod` floors at one second |
| AC-8 | Done | `TestISISLevelTimerChangeIsNotUnchanged` | `circuitParamsEqual` compares both containers whole |
| AC-9 | Done | `isis-per-level-hello-frr`, assertions 2 and 3 | FRR 10.3.1 reads 9 at Level-1 and 120 at Level-2 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestISISLevelHelloTimersResolution` | Done | `internal/plugins/isis/level_hello_timers_test.go` | 7 subtests, not the 5 the plan wrote |
| `TestISISPerLevelHelloTimersReachTheWire` | Done | `internal/plugins/isis/level_hello_timers_test.go` | |
| `TestISISLevelTimerChangeIsNotUnchanged` | Done | `internal/plugins/isis/level_hello_timers_test.go` | |
| `TestISISPerLevelHelloSchedulesOnBroadcast` | Done | `internal/plugins/isis/circuit/hello_level_timers_test.go` | |
| `TestISISPerLevelHelloScheduleSingleLevel` | Done | `internal/plugins/isis/circuit/hello_level_timers_test.go` | |
| `TestISISPerLevelHoldingTimeInLANIIH` | Done | `internal/plugins/isis/circuit/hello_level_timers_test.go` | |
| `TestISISSendHelloRefusesUnformedLevel` | Done | `internal/plugins/isis/circuit/hello_level_timers_test.go` | |
| `TestISISP2PRunsOneHelloSchedule` | Done | `internal/plugins/isis/circuit/hello_level_timers_test.go` | |
| `TestISISP2PL2OnlyTakesItsOwnTimers` | Done | `internal/plugins/isis/circuit/hello_level_timers_test.go` | |
| `TestISISHelloScheduleNeverZeroPeriod` | Done | `internal/plugins/isis/circuit/hello_level_timers_test.go` | |
| `TestISISPerLevelHelloNeighborTable` | Done | `internal/le/interoplab/bgp/bgp_test.go` | Covers `parseISISNeighbors` / `upAtLevel` |
| `isis-per-level-hello-frr` | Done | `test/interop/scenarios/isis-per-level-hello-frr/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/isis/circuit/circuit.go` | Done | `LevelTimers`, `Config.Level1`/`.Level2`, `timers`, `holdTime` |
| `internal/plugins/isis/circuit/runtime.go` | Done | `HelloSchedule`, `HelloSchedules`, `helloPeriod`, `SendHello(level)`, `sendLANHello` |
| `internal/plugins/isis/circuit/hello.go` | Done | Both builders take the level's holding time |
| `internal/plugins/isis/circuits.go` | Done | `levelHelloTimers`, the per-schedule tickers |
| `internal/plugins/isis/server.go` | Done | `circuitParamsEqual` |
| `internal/plugins/isis/yang/ze-isis-conf.yang` | Done | Six help strings |
| `docs/architecture/isis/isis-5-adjacency.md` | Done | New decision section, plus the closure's citation repoint |
| `docs/architecture/isis/isis-4-component-config.md` | Changed | Named UNAFFECTED in the plan and confirmed so at closure: no edit |
| `internal/le/interoplab/bgp/check_isis.go`, `check_special.go` | Done | Checker and registration |
| `internal/plugins/isis/circuit/hello_level_timers_test.go` | Done | Created |
| `internal/plugins/isis/level_hello_timers_test.go` | Done | Created |
| `test/interop/scenarios/isis-per-level-hello-frr/{ze,frr}.conf` | Done | Created |
| `internal/plugins/isis/auth_wiring.go`, `circuit/runtime_test.go`, `docs/architecture/isis/isis-10-auth.md`, `docs/functional-tests.md` | Changed | Not in the plan. Closure repairs: the RFC 5303 mis-citation and the seven-scenario count |

### Audit Summary
- **Total items:** 38 (5 requirements, 9 ACs, 12 tests, 12 planned files)
- **Done:** 36
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (`isis-4-component-config.md` confirmed unaffected; four unplanned files edited by the closure's repairs, recorded in Deviations and the Mistake Log)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A circuit runs a fast Level-1 hello and a slow Level-2 hello | interop | `isis-per-level-hello-frr` assertion 3: the Level-1 adjacency is still Up after a 20-second settle against the 9-second holding time it advertised, which only a 3-second Level-1 period sustains. The circuit-wide period is 10 seconds |
| A circuit advertises a different holding time at each level | interop | `isis-per-level-hello-frr` assertion 2: FRR 10.3.1's `show isis neighbor` reports a Level-1 holdtime at or under 15 and a Level-2 holdtime at or over 60, from a circuit whose circuit-wide pair advertises 30 at both. The RED run read 28 at BOTH levels |
| The four leaves stop being inert | data correctness | `TestISISPerLevelHelloTimersReachTheWire` decodes the IIH the engine sent and asserts holding time 9 at Level-1 and 60 at Level-2, plus the destination multicast group per level. RED against a `levelHelloTimers` that ignores the override: `l1 IIH holding time = 30, want 9` |
| A committed per-level change is not read as unchanged | functional | `TestISISLevelTimerChangeIsNotUnchanged`, RED against a `circuitParamsEqual` without the two container comparisons |
| The schema stops contradicting the code | data correctness | `grep -c "stored and not acted on" internal/plugins/isis/yang/ze-isis-conf.yang` answers 0 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Nothing | Every AC has product code and a test, and every planned file landed | - |

Two boundaries the spec DECLARED as out of scope stay out of scope and are not
work this spec left undone. Both are in Known Limitations, both predate this
change and cover the circuit-wide leaves identically, and neither is a
regression this spec introduced: applying a parameter change to a RUNNING
circuit (journalled at `plan/journal/unwired-feature.md`, 2026-09-06, because it
is one decision about which parameters can be updated without flapping an
adjacency), and making `show isis interface` report per level (one decision
covering all four override kinds).

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/isis-per-level-hello-timers-d64e7b3f-bfdc-4614-8db4-f12043eb77cc.md`, 18 files, verdict=clean |
| `./le spec session review check` | `review_gate: OK (0 code files, clean, hashes match ...)` |
| Rounds | 2. Round 1 found the two ISSUEs below; round 2 over the fixes found nothing above NOTE |
| Reviewer lenses used | wiring + removed-behavior audit; RFC conformance + citation verification against `rfc/full/`; documentation drift + Go style pass over every changed file |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | "RFC 5303 sec 3" cited for the level-agnostic point-to-point IIH. RFC 5303 defines TLV 240; `grep -n -i level rfc/full/rfc5303.txt` answers only the RFC 2119 boilerplate, so the document says nothing about levels. Four sites added by this change, four pre-existing | `circuit/circuit.go` `Config.Level1`, `circuit/runtime.go` `HelloSchedules` and `sendP2PHello`, `circuit/hello.go` `buildP2PHello`, `auth_wiring.go` `verifyFrame`, `circuit/runtime_test.go`, `circuit/hello_level_timers_test.go`, `docs/architecture/isis/isis-5-adjacency.md`, `docs/architecture/isis/isis-10-auth.md` | Repointed at RFC 1195 section 5.3, whose subsection list names the LAN IIH once per level (5.3.1, 5.3.2) and the point-to-point IIH once with no level in its name (5.3.3). Journal row in `plan/journal/reference-checked-claim-unchecked.md` |
| 2 | ISSUE | `docs/functional-tests.md` enumerated "The seven FRR interop scenarios" and listed seven stems. Nine tracked scenarios exist under `test/interop/scenarios/isis-*`, and this change added the ninth | `docs/functional-tests.md`, the IS-IS interop paragraph | Rewritten to name nine, with a sentence each for `isis-max-metric-frr` and `isis-per-level-hello-frr`, and a source anchor on `check_isis.go` |

NOTEs recorded and not fixed: `secondC` in `launchCircuitGoroutine` names a
channel by its ordinal plus its Go type letter; `circuitParamsEqual` compares
`Level2` on an L1-only circuit, where the container is ignored, which is
conservative rather than wrong. `./le repository check` reports one unwired
export in `internal/component/bgp/reactor/filter_delta.go`, an uncommitted file
belonging to another session and outside this diff.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/isis/circuit/hello_level_timers_test.go` | Yes | `ls -la` reports 8312 bytes |
| `internal/plugins/isis/level_hello_timers_test.go` | Yes | `ls -la` reports 10210 bytes |
| `test/interop/scenarios/isis-per-level-hello-frr/ze.conf` | Yes | `ls -la` reports 1550 bytes |
| `test/interop/scenarios/isis-per-level-hello-frr/frr.conf` | Yes | `ls -la` reports 632 bytes |
| `test/isis/isis-config.ci` | Yes | `ls -la` reports 1893 bytes, unchanged by this spec |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-6 | The override replaces the circuit-wide value at its own level; unset falls back | `./le job run label isis-close command go test ./internal/plugins/isis/...` -> `ok github.com/ze-software/ze/internal/plugins/isis 69.647s`, which runs `TestISISLevelHelloTimersResolution`'s 7 subtests |
| AC-2, AC-3, AC-4, AC-5, AC-7 | One schedule per level, per-level holding time, one P2P schedule, the refusal, the period floor | The same run -> `ok github.com/ze-software/ze/internal/plugins/isis/circuit 0.023s`, which runs the eight `hello_level_timers_test.go` tests |
| AC-8 | Reconcile sees a per-level timer change | The same run; `TestISISLevelTimerChangeIsNotUnchanged` is in the `isis` package result above |
| AC-9 | FRR reads one holding time per level | `checkISISPerLevelHelloTimers` (`internal/le/interoplab/bgp/check_isis.go`) is registered in `check_special.go` for `isis-per-level-hello-frr`; its table parser is covered by `TestISISPerLevelHelloNeighborTable`, run fresh: `--- PASS: TestISISPerLevelHelloNeighborTable (0.00s)` |
| Deliverable 3 | The schema no longer says the leaves are inert | `grep -c "stored and not acted on" internal/plugins/isis/yang/ze-isis-conf.yang` answers `0` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `interfaces/interface/eth0/level-1/hello-interval` in the config text | `internal/plugins/isis/level_hello_timers_test.go` (`TestISISPerLevelHelloTimersReachTheWire`) | Yes. Read in full: it calls `parseISISConfig` on the config TEXT, `eng.openCircuits()`, `eng.buildCircuit`, then `c.HelloSchedules()` and `c.SendHello(level)`, and decodes the captured PDU with `packet.DecodePDU`, asserting the Holding Time field and the destination MAC from `transport.MulticastMACForLevel`. No stub stands in for the parser, the resolver or the encoder |
| The same config in a running daemon on a LAN with FRR | `test/interop/scenarios/isis-per-level-hello-frr/ze.conf` + `checkISISPerLevelHelloTimers` | Yes. Read in full: `ze.conf` sets circuit-wide 10/3 with level-1 3/3 and level-2 30/4, and the checker parses FRR's `show isis neighbor` and bounds the Level-1 holdtime at 15 and the Level-2 holdtime at 60, either of which the circuit-wide 30 fails |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestISISPerLevelHelloTimersReachTheWire` asserts the destination MAC per level against `transport.MulticastMACForLevel`, so the two levels really are separate PDUs to separate groups |
| A-2 | confirmed | `TestISISP2PRunsOneHelloSchedule` asserts a one-element slice on an L1L2 P2P circuit. Its BASIS was corrected at closure: `rfc/full/rfc1195.txt` section 5.3 rather than `rfc/short/rfc5303.md`, which says nothing about levels |
| A-3 | confirmed | The RED run of `isis-per-level-hello-frr` read Holdtime 28 on both rows of FRR's table; the green run read the two overrides apart |
| A-4 | confirmed | `TestISISLevelHelloTimersResolution` covers the unset, half-set and maximum cases, and `ze-isis-conf.yang` bounds both leaves away from zero (`range "1..65535"`, `range "1..255"`) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 12 Yes: `isis-5-adjacency.md` gains the per-level timer decision | The page's new section names `HelloSchedules`, `SendHello`, `p2pPreferredLevel` and `levelHelloTimers`, each read at its producing function | Yes |
| Row 10 Yes: test infrastructure changed | `docs/architecture/testing/interop.md` names no scenario (`grep -n isis` answers nothing), so it needed no edit. `docs/functional-tests.md` DID need one and was wrong: fixed, see Findings fixed #2 | Yes, after repair |
| Row 16 Yes: changed files under existing source anchors | `grep -rn "source: internal/plugins/isis" docs/` names `spf/`, `lsdb/`, `register.go`, `packet/`, `transport/`, `redistribute/` and `own_lsp_conflict.go`. None is a file this change touched, except `server.go` through the dispatcher anchor in `docs/architecture/isis/isis-4-component-config.md`, whose claim is the PDU dispatcher and circuit lifecycle and is unchanged | Yes |
| Row 2 No: config syntax unchanged | `internal/le/site/testdata/published-configuration.md` and `published-yang-config-tree.json` render the YANG `description`, not `ze:help`. The four `description` strings are byte-identical before and after, so neither golden needed regeneration | Yes |
| Row 6 No: no user guide page | `grep -n "hello\|hold" docs/guide/isis.md` returns five hits, all about key chains and the `show` column list. No timer section exists | Yes |
| Row 9 No: no RFC row moves | No requirement id is added or re-levelled. The one citation the change adds is explanatory, and closure corrected it from RFC 5303 to RFC 1195 section 5.3 | Yes |
| `./le doc check verify` | Run at closure; result recorded with the verification gate below | Yes |

## Core Insight

The advertised holding time must come from the period the IIH REALLY goes out
at, not from the level whose identity the PDU otherwise carries. On a
point-to-point circuit those two are different facts: the signing chain follows
the NEGOTIATED adjacency level and can change once a neighbor is heard, while
the holding time follows the SCHEDULE and is fixed at circuit build. Reading one
off the other would have advertised a holding time no timer produced.
