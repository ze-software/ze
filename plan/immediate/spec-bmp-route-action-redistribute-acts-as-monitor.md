# Spec: bmp-route-action-redistribute-acts-as-monitor

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Research note (2026-10-13, HEAD 6b2f7cfa40).** The Task text below was
written 2026-09-06. Two of its facts moved since and are corrected here rather
than in place: the producer is now `(*BMPPlugin).applyReceiverConfig` in
`internal/component/bgp/plugins/bmp/receiver.go` (it was `startReceiver` in
`bmp.go`), and the YANG `description` of the leaf now admits the gap ("redistribute
is accepted by the schema and is not implemented"), while the `ze:help` text the
editor shows for the value still promises best-path entry. The defect is open at
HEAD: the warning is still the only branch on the value. No other session holds
this spec (no state file, no commit names it), and neither `dce906c381` (Peer
Down only for a peer reported up, sender side) nor
`plan/immediate/spec-bmp-sflow-export-rfc-defects.md` (Peer Down reasons, sFlow
pool, all sender side) touches `receiver.go`, the receiver YANG, or
`route_action_test.go`.

**What the operator is promised.**
`internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` declares
`leaf route-action` under the BMP receiver configuration as an enumeration of
two values, default `monitor`, described as "Action for received BMP Route
Monitoring messages". The `monitor` value is described as "Store in BMP RIB for
visibility (show bmp rib), isolated from BGP best-path and FIB". The
`redistribute` value is described as "Store in BMP RIB and redistribute into BGP
best-path selection". A reader takes the second value to mean routes learned
from a monitored router enter best-path selection and can be installed.

**What Ze does instead.** The leaf is live and one of its two values lies.
`(*BMPPlugin).startReceiver` (`internal/component/bgp/plugins/bmp/bmp.go`) is
the producer. It tests `cfg.RouteAction == "redistribute"`, writes the warning
`bmp: route-action redistribute is not yet implemented, using monitor`, then
logs `bmp: receiver route-action: monitor` unconditionally and starts the
listeners. No branch downstream of that test reads the value, so the received
routes go to the BMP RIB and reach neither best-path selection nor the FIB. An
operator who writes `redistribute` gets `monitor`, with one warning line in the
log as the only signal.

The schema is what makes this the operator's problem rather than a missing
feature. The enumeration lists `redistribute` as a valid value, so the config
editor completes it, the commit succeeds, and `show configuration` reads it
back. Ze accepts a configuration and then does something else, which is the test
`plan/immediate/README.md` applies.

**What closing it means.** The leaf is live, so this is not a whole-container
question: one enum value is wrong and the other is right. The implementer
chooses between implementing redistribution and refusing the value at commit.
The refusal here is narrower than `unimplementedVRFValidator`
(`internal/component/config/validators.go`), which refuses a leaf outright: this
one refuses a single enum value, so the design has to decide whether the
refusal is a `ze:validate` binding or the removal of the enum value from the
schema. Removing the value is the simpler shape and it costs the operator the
ability to declare an intent Ze will honor later. Neither is obviously right,
and the choice turns on whether BMP redistribution is wanted. If it is, the
refusal is the placeholder and this spec stays open, which is the shape
`plan/spec-vrf.md` records for `vrf`. Implementing it is the larger of the two:
routes learned over BMP have a different trust and loop-prevention story from
routes learned over a BGP session, and the design owes that answer before any
code.

**Why this is not one spec with the sender leaf.** The other unowned BMP leaf,
`bmp/statistics-timeout`, was covered by
spec-bmp-statistics-timeout-sends-no-report, closed 2026-10-09. This spec is
the BMP receiver, deciding what Ze does with routes it is told about. That one
is the BMP sender, deciding what Ze emits to a collector. They share no code and
no direction, so one change cannot fix both.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/guide/bmp.md` - the page that documents the receiver and the leaf
  → Constraint: the receiver field table row for `route-action` says `redistribute` is "future: also enter best-path", and the Looking Glass Integration section ("The `route-action` leaf controls future behavior") lists it as "not yet implemented". Both sentences become false the moment the value is gone, so they are edited in the same phase as the YANG.
  → Constraint: the same section states "These routes are visible through `show bmp rib` and looking glass endpoints but never enter BGP best-path selection or the FIB." That is the `monitor` promise and it stays; it is proven by `test/plugin/bmp-lg-bestpath-isolation.ci` and `TestBMPRoutesExcludedFromBestPath`.
- [ ] `docs/architecture/plugin/rib-storage-design.md` - two-level `ribInPool` keyed by source protocol
  → Constraint: protocol namespaces other than BGP are stored but never offered as best-path candidates; the receiver change must not touch `internal/component/bgp/plugins/rib`.
- [ ] `internal/component/config/validators.go`, `unimplementedVRFValidator` (precedent named by the Task)
  → Decision: its comment records that deleting a LEAF does not refuse a config, because `ValidateTreeAllModules` never emits `ErrTypeUnknown` (`validate_sections.go`). Deleting an enum VALUE is different: `validateEnumeration` (`internal/component/config/yang/validator.go`) and the enum arm of `ValidateLeafValue` refuse a value outside the enumeration natively, with the message "invalid value for <leaf> ... expected one of <values>" (proven for `asn4` by `test/parse/asn4-refuses-boolean-spelling.ci`). So removing the value refuses the config with no custom validator.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7854.md` and `rfc/full/rfc7854.txt` - BMP, the role Ze fills as receiver (monitoring station)
  → Constraint: RFC 7854 defines no use of received routes by the station. Section 1: "BMP provides access to the Adj-RIB-In of a peer on an ongoing basis and a periodic dump of certain statistics the monitoring station can use for further analysis." Section 2 defines what Route Monitoring carries: "Adj-RIB-In: As defined in [RFC4271], "The Adj-RIBs-In contains unprocessed routing information that has been advertised to the local BGP speaker by its peers."" That is another router's unprocessed input, not routes offered to Ze.
  → Constraint: Section 11 (Security Considerations): "Unless a transport that provides mutual authentication is used, an attacker could masquerade as the monitored router and trick a monitoring station into accepting false information, or they could masquerade as a monitoring station and gain unauthorized access to BMP data." Ze's receiver has no authentication; feeding received routes into best-path would turn that "false information" into installed routes.
  → Constraint: Section 3.2: "No BMP message is ever sent from the monitoring station to the monitored router." The station has no channel to refuse or bound what it is fed.
  → Decision: no `rfc/short/rfc7854.md` requirement row maps to `route-action`; no row, `Support` value, or `docs/features/rfc-status.md` line changes. The RFC is silent on station-side redistribution, so this is a Ze config-truth defect, not an RFC defect.

**Key insights:** (minimal context to resume after compaction)
- Producer: `applyReceiverConfig` (`receiver.go`) is the ONLY reader of `RouteAction`; it logs a warning and nothing else branches on the value.
- `monitor` is real: `processRouteMonitoring` (`bmp.go`) calls `InjectWireRoute(protocolBMP, ...)`; `handleInjectWireRoute` (`rib/rib_inject.go`) stores into `ribInPool[bmpProtocolID]`; best-path candidates (`gatherPrefixCandidatesLocked`, `gatherKeyCandidatesLocked`, `rib_commands.go`) iterate `r.bgpPeers` only.
- Removing `enum redistribute` from the YANG refuses the config natively; a custom validator is not needed.
- No RFC-tagged test changes under the recommended design.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/bmp/receiver.go` (128L) - `parseReceiverConfig` decodes `{"environment":{"bmp":{...}}}` with plain `json.Unmarshal` (unknown keys ignored); `applyReceiverConfig` warns "bmp: route-action redistribute is not yet implemented, using monitor" when `cfg.RouteAction == "redistribute"`, stores `max-sessions`, then reconciles listeners. Called at Stage-2 configure, reload apply, and rollback.
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - `receiverConfig.RouteAction string` with json tag `route-action`; `processRouteMonitoring` injects every Route Monitoring UPDATE body under `protocolBMP`, with no reference to `RouteAction`.
- [ ] `internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` - `environment/bmp/route-action`, enumeration `monitor` | `redistribute`, default `monitor`; `ze:help` of `redistribute` is "Store in BMP RIB and redistribute into BGP best-path selection"; `description` already says redistribute "is not implemented: Ze writes a warning in the log and acts as monitor".
- [ ] `internal/component/bgp/plugins/rib/rib_inject.go` - `handleInjectWireRoute` stores BMP routes in the protocol namespace, refuses protocol `bgp`, applies RFC 7606 validation.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go`, `rib_pipeline_best.go` - candidates come from `bgpPeers` only, so BMP routes never win best-path.
- [ ] `internal/component/bgp/plugins/bmp/route_action_test.go` - three tests: two exercise `processRouteMonitoring` with a nil plugin (no assertion beyond no-panic); `TestReceiverConfig_RouteActionDefault` asserts an empty struct field is empty, which proves nothing about the YANG default. None carries an `RFC requirement:` tag.
- [ ] `test/parse/bmp-config.ci`, `test/plugin/bmp-lg-ingest.ci`, `test/plugin/bmp-lg-bestpath-isolation.ci` - receiver parse, ingest, and best-path isolation; none writes `route-action`. None carries an `RFC requirement:` tag.
- [ ] `test/interop/scenarios/bmp-locrib-receiver-frr/ze.conf` - writes `route-action monitor;` (stays valid). Its checker entry `scenarioLocRIBReceiverFRR` (`internal/le/interoplab/bgp/checkers.go`) waits on `show bmp peers` only.
- [ ] `internal/le/site/testdata/published-configuration.md`, `published-yang-config-tree.json` - carry the `redistribute` help text, but they are snapshots of the PUBLISHED site used as fixtures (`configurationPaths` in `config_test.go` copies the JSON in as input), not generated from the live YANG, so the YANG change does not redden them.

**Behavior to preserve:** (unless the user explicitly said to change it)
- `route-action monitor;` and an absent `route-action` keep committing, and Route Monitoring keeps landing in the BMP RIB, isolated from best-path and FIB.
- `show bmp rib`, the looking-glass BMP endpoints, `show bgp rib` excluding BMP routes.
- Listener reconcile in `applyReceiverConfig` (add, keep, close) and the `max-sessions` store.
- `test/interop/scenarios/bmp-locrib-receiver-frr/ze.conf` loads unchanged.

**Behavior to change:** (only what the user asked for)
- `route-action redistribute` is refused at parse/validate/commit instead of committing and acting as `monitor`.
- The runtime warning, which can no longer fire, is deleted, and so is the `RouteAction` field nobody reads.
- Help text and the guide stop promising best-path entry.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config text `environment { bmp { route-action <value>; } }`, from a config file at daemon start, `ze config validate`, the CLI editor commit, or a reload.
- Format at entry: a YANG enumeration leaf in module `ze-bmp-conf`.

### Transformation Path
1. Parse: the schema-driven parser checks the leaf value against the enumeration (`ValidateLeafValue`, `internal/component/config/schema.go`); after the change `redistribute` fails here.
2. Validate: `ValidateTreeAllModules` runs `validateEnumeration` (`internal/component/config/yang/validator.go`) over the tree; same refusal on the commit path.
3. Delivery: `ExtractConfigSubtree` hands the plugin `{"environment":{"bmp":{...}}}`; `parseReceiverConfig` decodes it; `applyReceiverConfig` reconciles listeners. After the change nothing in the plugin reads `route-action`.
4. Data plane (unchanged): `processRouteMonitoring` → `InjectWireRoute(protocolBMP)` → `handleInjectWireRoute` → `ribInPool[bmpProtocolID]`, outside best-path candidates.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → Plugin | JSON subtree `environment/bmp` via Stage-2 configure and reload config-apply | Yes, `parseReceiverConfig` read at HEAD |
| Plugin → RIB | `InjectWireRoute(protocol, peerKey, updateBody)` over the bridge; unchanged | Yes, `handleInjectWireRoute` read at HEAD |

### Integration Points
- YANG enumeration validation (native) - the refusal; no new validator, no registry entry.
- `docs/guide/bmp.md` - the page that describes the leaf.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | refusal happens in the schema layer every config path already runs |
| No unintended coupling (components stay isolated) | Yes | only the BMP plugin's own YANG and Go change |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses native enum refusal; no custom validator duplicating it |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no data-path change |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | nothing is added to `internal/component/config/validators_register.go` (the alternative B would have added a central entry) |

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
| A-1 | The owner does not want BMP redistribution built now, so refusing the value is the fix rather than a placeholder | RFC 7854 defines no station-side use of received routes (Sections 1, 2) and warns of false information without mutual authentication (Section 11); `plan/README.md` immediate-bucket test is "Ze accepts a configuration and then does something else" | If redistribution is wanted, approach C replaces this spec's design and a trust and loop-prevention design is owed first; the refusal still lands as the interim per Q-1 | Owner answer to Q-1 | unvalidated |
| A-2 | A value outside the enumeration is refused on EVERY config path: file load at start, `ze config validate`, editor commit, reload | `test/parse/asn4-refuses-boolean-spelling.ci` proves it for `asn4` via `ze config validate`; `validateEnumeration` is in the tree walk | A path that skips enum checking would still accept `redistribute` and the plugin would then silently ignore it | AC-1 `.ci` (validate), AC-2 `.ci` (daemon start refuses), AC-3 (reload refused) | unvalidated |
| A-3 | No config in the repository, test, or interop scenario writes `route-action redistribute` | `grep -rn "route-action" test etc docs internal` at HEAD: only `bmp-locrib-receiver-frr/ze.conf` (`monitor`) and the guide | A fixture would go red at implementation | Re-run the grep in Phase 1 | unvalidated |
| A-4 | The site fixtures `internal/le/site/testdata/published-*.{md,json}` are snapshots and do not follow the live YANG | `configurationPaths` copies `published-yang-config-tree.json` in as input | `TestTheConfigurationMirrorReadsAsThePublishedMirror` would redden | Run `./le go test package internal/le/site` scoped after the YANG edit | unvalidated |
| A-5 | Nothing outside the BMP plugin reads `route-action` or `receiverConfig.RouteAction` | grep for `RouteAction` and `route-action` at HEAD: only `bmp.go`, `receiver.go`, `route_action_test.go` (the `redistevents.RouteAction` hits are an unrelated type) | Deleting the field breaks a build | `gopls references` on the field before deleting it | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator's stored config holding `route-action redistribute` stops loading after upgrade, so the daemon does not start | parse error naming `route-action` at start | Ze is pre-release, so no stored config is owed a migration (`ai/rules/pre-release.md`); the error names the leaf and the allowed value, and the guide says to delete the line or write `monitor` |
| R-2 | The native enum error says "expected one of monitor" but not WHY `redistribute` is gone, which `ai/rules/user-facing-errors.md` may judge insufficient | review of the AC-1 message | Q-2: if the owner wants the reason in the message, approach B (a `ze:validate` refusal with its own text) replaces approach A for the message, keeping the enum as is |
| R-3 | A one-value enumeration reads as pointless and a later cleanup deletes the leaf, which (per the `unimplementedVRFValidator` note) would make `route-action redistribute` load SILENTLY again | a diff that removes the leaf | The YANG `description` states why the leaf stays with one value; the AC-1 `.ci` goes red if the leaf is removed and the value is then silently accepted (it asserts exit 1) |
| R-4 | Deleting the warning branch and field leaves a comment elsewhere describing the warning | grep for "not yet implemented, using monitor" | Phase 2 greps and removes every occurrence (`ai/rules/stale-comments.md`) |
| R-5 | Reload with the value refused leaves the old config running, and the operator reads that as success | reload returns an error | AC-3 asserts the reload is refused with the message and the listener set is unchanged |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A config holding `route-action redistribute` is rejected (intended); if the refusal leaked to `monitor` or to the absent leaf, every BMP receiver config would be rejected and the receiver would not start |
| How is it reverted? | Single commit revert; no wire, storage, or config-format migration |
| Who else touches this path? | `plan/immediate/spec-bmp-sflow-export-rfc-defects.md` (sender side, `bmp_events.go`, no shared file); the BMP sender YANG block in the same `ze-bmp-conf.yang` file (different container, `bgp/bmp/sender`) |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate -` with `route-action redistribute` | → | `ValidateLeafValue` enum arm against the `ze-bmp-conf` enumeration | `test/parse/bmp-route-action-refuses-redistribute.ci` |
| `ze -` daemon start with the value in the config file | → | config load, enum refusal before any plugin starts | `test/parse/bmp-route-action-refuses-redistribute.ci` (second command) |
| SIGHUP reload to a config with the value | → | reload parse refused, running config kept | `test/reload/bmp-route-action-redistribute-refused.ci` |
| `ze config validate -` with `route-action monitor` and with the leaf absent | → | same enum arm accepts | `test/parse/bmp-route-action-refuses-redistribute.ci` (accepted counterparts) and `test/parse/bmp-config.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze config validate -` on a config whose `environment bmp` writes `route-action redistribute;` | exit 1; output contains "invalid value for route-action" and "expected one of" and names `monitor` as the only value. Proof: `test/parse/bmp-route-action-refuses-redistribute.ci`, first command |
| AC-2 | The same config as the daemon's startup config (`ze -`) | daemon exits non-zero before the BMP receiver listens; stderr carries the AC-1 refusal; no "bmp: receiver listening" line. Proof: same `.ci`, second command |
| AC-3 | Daemon running a valid BMP receiver config, then SIGHUP to a config that adds `route-action redistribute;` | reload refused with the AC-1 message on stderr; the daemon keeps running and the BGP peer session in the test stays up (exit 0 after SIGTERM). Proof: `test/reload/bmp-route-action-redistribute-refused.ci` |
| AC-4 | `route-action monitor;` written explicitly, and the leaf absent | both validate (exit 0). Proof: AC-1 `.ci` accepted counterparts; `test/parse/bmp-config.ci` unchanged |
| AC-5 | A Route Monitoring message for a prefix also learned from a real BGP peer, and for a prefix only BMP knows, under default config | BMP routes appear in `show bmp rib`, never as a best-path candidate, unchanged. Proof: `test/plugin/bmp-lg-bestpath-isolation.ci` and `TestBMPRoutesExcludedFromBestPath` pass unchanged |
| AC-6 | The editor completes values for `environment bmp route-action` | completion offers `monitor` only. Proof: `TestRouteActionEnumIsMonitorOnly` (reads the enumeration through `yangEnumValues`) |
| AC-7 | Source after the change | the string "route-action redistribute is not yet implemented" appears nowhere; `receiverConfig` has no `RouteAction` field; the YANG `ze:help` and `description` and `docs/guide/bmp.md` no longer say `redistribute` exists or is planned. Proof: grep in the Deliverables table, plus the build |
| AC-8 | Interop scenario `bmp-locrib-receiver-frr` (`ze.conf` writes `route-action monitor;`) | still loads and passes. Proof: `./le test integration` scenario run, recorded in the Goal Validation table |
| AC-9 | A Route Monitoring message reaches `processRouteMonitoring` (owner answer Q-3) | with a recording stand-in for the RIB injection, a well-formed UPDATE is injected exactly once under `protocolBMP` with its bytes unchanged, and a truncated UPDATE injects nothing. Proof: `TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB` and `TestProcessRouteMonitoring_ShortUpdate_Skipped` rewritten to assert both, each shown red with the injection call removed (first) or the length guard removed (second) |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | writes `route-action redistribute` expecting best-path entry | config text → parser enum check → refusal naming the leaf and `monitor` | `test/parse/bmp-route-action-refuses-redistribute.ci` |
| 2 | reloads a running daemon into that config | SIGHUP → reload parse → refusal, old config kept | `test/reload/bmp-route-action-redistribute-refused.ci` |
| 3 | runs a BMP receiver with the default and reads monitored routes | BMP TCP → `processRouteMonitoring` → `InjectWireRoute(protocolBMP)` → `show bmp rib`; best-path untouched | `test/plugin/bmp-lg-bestpath-isolation.ci` (unchanged) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRouteActionEnumIsMonitorOnly` (new) | `internal/component/bgp/plugins/bmp/yang_enum_agreement_test.go` | the `ze-bmp-conf` enumeration at `environment/bmp/route-action` is exactly `[monitor]` and its default is `monitor` (AC-6). Red before the YANG edit (two values) | |
| `TestReceiverConfig_RouteActionDefault` (DELETE) | `internal/component/bgp/plugins/bmp/route_action_test.go` | asserts an empty Go field is empty; reads a field this spec deletes and proved nothing about the YANG default, which `TestRouteActionEnumIsMonitorOnly` now proves. Not RFC-tagged. Deleting it is replacing a vacuous test with a real one, not weakening coverage; named here so the reviewer can check that | |
| `TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB`, `TestProcessRouteMonitoring_ShortUpdate_Skipped` | `internal/component/bgp/plugins/bmp/route_action_test.go` | unchanged; they do not read `RouteAction`. Their no-assertion shape is a pre-existing weakness, out of this spec's goal: one journal row (`plan/journal/`, class weak-test or equivalent after grep) | |

**RFC-tagged tests that change: none.** `grep "RFC requirement"` over `route_action_test.go`, `rib_inject_test.go`, `test/parse/bmp-config.ci`, `test/plugin/bmp-lg-ingest.ci` and `test/plugin/bmp-lg-bestpath-isolation.ci` returns nothing, and the new tests carry no tag because no RFC 7854 row governs station-side use of routes. No owner approval for an RFC-tagged test is owed under approach A. Under approach C (implement redistribution) that answer would have to be re-derived.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | the change touches an enumeration, no numeric input | N/A | N/A | N/A |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bmp-route-action-refuses-redistribute` (new) | `test/parse/bmp-route-action-refuses-redistribute.ci` | shape of `test/parse/asn4-refuses-boolean-spelling.ci`: stdin `redistribute` → `ze config validate -` exit 1 with "invalid value for route-action", "expected one of", "monitor" (no `:` in needles); stdin `monitor` and stdin with the leaf absent → exit 0; then `ze -` on the `redistribute` stdin exits non-zero with the same message and no "receiver listening" (AC-1, AC-2, AC-4) | |
| `bmp-route-action-redistribute-refused` (new) | `test/reload/bmp-route-action-redistribute-refused.ci` | shape of `test/reload/config-reload-invalid-validator.ci`: daemon up with a BMP receiver and one test peer; `action=rewrite` to a config adding `route-action redistribute;`, `action=sighup`, then `sigterm`; stderr carries the refusal, the peer saw no NOTIFICATION, exit 0 (AC-3) | |
| `bmp-lg-bestpath-isolation`, `bmp-lg-ingest`, `bmp-config` | `test/plugin/`, `test/parse/` | unchanged, must stay green (AC-4, AC-5) | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bmp-locrib-receiver-frr` (existing, unchanged) | `test/interop/scenarios/bmp-locrib-receiver-frr/` | FRR `bgpd -M bmp` as monitored router, Ze as station | its `ze.conf` writes `route-action monitor;` and must still load and reach `show bmp peers` (AC-8). pmacct is NOT the peer here: pmacct `pmbmpd` is a COLLECTOR (`docs/architecture/testing/interop.md`) and reads Ze's SENDER stream, which this spec does not touch | |

**Interop applicability.** `ai/rules/interop-and-goal-validation.md` lets interop be omitted for "a config-only feature with no protocol impact". This change refuses a config value and changes no byte Ze reads or writes, so no NEW scenario is owed; AC-8 runs the existing receiver scenario to prove the surviving value still loads. Vacuity walk for the new `.ci` files: revert the YANG edit (restore `enum redistribute`), rebuild `ze`, confirm both `.ci` go red (validate exits 0; reload is accepted), restore, confirm green, record the red output in the spec.

### Goal Validation
| Goal (from Task) | Evidence |
|------------------|----------|
| Ze no longer accepts a `route-action` value it does not honor | `test/parse/bmp-route-action-refuses-redistribute.ci` and `test/reload/bmp-route-action-redistribute-refused.ci` green, with the recorded red from the revert walk |
| `monitor` still does what its help says (BMP RIB only, no best-path, no FIB) | `test/plugin/bmp-lg-bestpath-isolation.ci` green; `bmp-locrib-receiver-frr` scenario run green |
| No surface still promises redistribution | Deliverables grep (AC-7) returns nothing in `internal/`, `docs/`, YANG |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` - delete `enum redistribute`; rewrite the leaf `description` to say `monitor` is the only action, what it does, and why the leaf stays with one value (a removed leaf would be skipped silently, a removed value is refused); keep `ze:help` of `monitor`
- `internal/component/bgp/plugins/bmp/receiver.go` - delete the `RouteAction == "redistribute"` warning in `applyReceiverConfig`
- `internal/component/bgp/plugins/bmp/bmp.go` - delete `receiverConfig.RouteAction`
- `internal/component/bgp/plugins/bmp/route_action_test.go` - delete `TestReceiverConfig_RouteActionDefault`
- `internal/component/bgp/plugins/bmp/yang_enum_agreement_test.go` - add `TestRouteActionEnumIsMonitorOnly`
- `docs/guide/bmp.md` - receiver field table row and the Looking Glass Integration paragraph (`// Design:` header of `receiver.go` names this page)
- `plan/journal/unwired-feature.md` - the 2026-09-04 row naming `bmp/route-action` records the fix (append, do not rewrite the row's history)

## Files to Create
- `test/parse/bmp-route-action-refuses-redistribute.ci` - AC-1, AC-2, AC-4
- `test/reload/bmp-route-action-redistribute-refused.ci` - AC-3

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang`: one enum value removed, description rewritten; no new node |
| YANG validation constraints | Yes | the `enumeration` itself is the constraint that refuses `redistribute` |
| YANG custom validators | No | native enum refusal suffices (approach A); approach B would add one |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | Yes | automatic from the enumeration; AC-6 proves `monitor` only |
| Functional test for new RPC/API | N-A | no RPC; config refusal is covered by the two new `.ci` files |
| Pipe completeness | N-A | no output command |
| Env var registration | N-A | `environment/bmp` registers no `ze.bmp.*` env var at HEAD (grep of the plugin and `internal/core/env`), and this spec adds none |
| Doctor check for runtime dependencies | N-A | no runtime dependency added |
| Prometheus counters/metrics | N-A | no observable state added |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | refusal of a value; `docs/features.md` and `features/` carry no `route-action` text (grep at HEAD) |
| 2 | Config syntax changed? | Yes | `docs/guide/bmp.md` receiver field table (the leaf lives only there); `docs/guide/configuration.md` and `docs/architecture/config/syntax.md` do not mention `route-action` (grep) |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | plugin behavior on valid configs is unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md`: receiver field table and Looking Glass Integration section |
| 7 | Wire format changed? | No | no byte changes |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no RFC 7854 row governs station-side route use; RFC scope is frozen and no summary changes |
| 10 | Test infrastructure changed? | No | two `.ci` in existing directories with existing directives |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` does not mention BMP redistribution (verify by grep in Phase 3) |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/bmp.md` (declared by `receiver.go` `// Design:`; source anchors on `bmp.go` and `ze-bmp-conf.yang`): updated. `docs/architecture/config/yang-config-design.md` (declared by `yang_enum_agreement_test.go`): unaffected, it describes how enum and validator layers work, not which BMP values exist; the new test follows its existing pattern. `docs/architecture/core-design.md` (declared by `bmp.go`): unaffected, it names no BMP receiver value or `route-action` (re-grep in Phase 3) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/bmp.md` examples; verify none writes `route-action redistribute` (none at HEAD) |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

Steps assume approach A and the owner's answer to Q-1 being "refuse". A different answer to Q-1 or Q-2 changes Phase 2.

1. **Phase: Wiring (MANDATORY FIRST)** -- the failing tests through the real entry points
   - Tests: `test/parse/bmp-route-action-refuses-redistribute.ci`, `test/reload/bmp-route-action-redistribute-refused.ci`, `TestRouteActionEnumIsMonitorOnly`
   - Files: the two `.ci`, `yang_enum_agreement_test.go`; re-run the A-3 grep
   - Verify: all three red against HEAD (validate exits 0, reload accepted, enum has two values); paste output
2. **Phase: Schema** -- delete `enum redistribute`, rewrite the leaf `description`, edit `docs/guide/bmp.md` in the same phase
   - Tests: the three above go green; `test/parse/bmp-config.ci`, `test/plugin/bmp-lg-bestpath-isolation.ci`, `test/plugin/bmp-lg-ingest.ci` stay green; scoped `internal/le/site` package test stays green (A-4)
   - Files: `ze-bmp-conf.yang`, `docs/guide/bmp.md`
3. **Phase: Dead code** -- delete the warning branch, `receiverConfig.RouteAction`, `TestReceiverConfig_RouteActionDefault`
   - Tests: scoped `internal/component/bgp/plugins/bmp` package test green; `gopls references` on the field empty before deletion (A-5); grep for the warning text empty (R-4)
   - Files: `receiver.go`, `bmp.go`, `route_action_test.go`
4. **Phase: Proof** -- revert walk on the YANG edit (rebuild `ze`, both `.ci` red, restore, green, record); run `bmp-locrib-receiver-frr` (AC-8); append the fix to the `plan/journal/unwired-feature.md` row; add the weak-test journal row named in the TDD plan

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | the refusal names `route-action`, the rejected value, and `monitor`; `monitor` and the absent leaf still commit |
| Naming | no new names; the new `.ci` files follow the `<subject>-refuses-<value>` shape of `asn4-refuses-boolean-spelling.ci` |
| Data flow | refusal in the schema layer only; the BMP plugin holds no knowledge of the removed value |
| Rule: no-layering | the warning and field are deleted, not kept beside the refusal |
| Rule: stale-comments | no comment, help, description or guide sentence still names `redistribute` as a value |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `redistribute` gone from schema, code and guide | `grep -rn "redistribute" internal/component/bgp/plugins/bmp docs/guide/bmp.md` returns only unrelated `redistribute_egress` mentions |
| warning text gone | `grep -rn "not yet implemented, using monitor" internal` returns nothing |
| refusal through real entry points | the two new `.ci` green, with the recorded red from the revert walk |
| `monitor` unchanged | `test/plugin/bmp-lg-bestpath-isolation.ci` green; `bmp-locrib-receiver-frr` green |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | the refusal holds on every config path (start, validate, commit, reload: A-2) so no path can enable a value the plugin silently ignores |
| Fail-open | the change removes a path that could later fail open: an unauthenticated BMP feed (RFC 7854 Section 11) can no longer be configured toward best-path by a leftover value |

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
- Removing an unhonored ENUM VALUE refuses it natively; removing an unhonored LEAF does not (unknown nodes are skipped by `ValidateTreeAllModules`). The cheap fix for a one-value lie is the value, never the leaf.
- `monitor` isolation is structural, not a branch on the leaf: best-path candidates are gathered from `bgpPeers` only, so no config value could have turned redistribution on without new RIB code.

## RFC annotations owed by this spec
None. No code added here implements or calls an RFC 7854 obligation; the removed branch and field carried no RFC comment.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A (recommended): remove `enum redistribute` from the YANG; native enum refusal; delete warning and field | B: keep the value and refuse it with a `ze:validate` custom validator (the `unimplementedVRFValidator` shape). C: implement redistribution. D: delete the whole leaf | A is the minimum change and adds no central entry: B needs a validator registered in `internal/component/config/validators_register.go` (a central list) and keeps completing a value Ze refuses; its one gain is a refusal message that says WHY (Q-2). C is a feature, not a fix: RFC 7854 defines no station-side route use, Section 11 says an unauthenticated feed can "trick a monitoring station into accepting false information", and BMP Route Monitoring carries another router's pre-policy Adj-RIB-In with next hops that belong to that router's peers; a trust, next-hop and loop-prevention design is owed before any code (Q-1). D looks simpler but, per the `unimplementedVRFValidator` note, an unknown leaf is skipped by `ValidateTreeAllModules`, so `route-action redistribute` could load silently again (the parser's handling of an unknown leaf was not verified here, and A does not depend on it) |

### Triple Challenge
| Challenge | Answer |
|-----------|--------|
| Simplicity | One enum value deleted, one branch and one field deleted, two `.ci` and one unit test. Nothing simpler refuses the value |
| Uniformity | Same refusal shape as `asn4` boolean spellings (native enum refusal, `test/parse/asn4-refuses-boolean-spelling.ci`) |
| Performance | No runtime path changes; one branch less in `applyReceiverConfig` |

### Failure Modes
Recorded as R-1 to R-5 (stored config stops loading, message lacks the reason, later leaf deletion reopens a silent path, stale comments, reload refusal misread). Concurrent access: none; the change is schema-time.

## Owner Questions
| ID | Question | Recommended answer | Why it matters |
|----|----------|--------------------|----------------|
| Q-1 | Is BMP redistribution (routes learned as a monitoring station entering best-path and the FIB) wanted at all? If yes, it becomes its own spec in `plan/` with a trust and loop-prevention design, and this spec's refusal is the placeholder until it lands. If no, the refusal is the end state | No. RFC 7854 frames the station as an observer (Sections 1, 2) and warns it can be fed false information without mutual authentication (Section 11) | decides whether the refusal is final and whether a feature spec is created. **Answered by the owner, 2026-10-13: "NO BMP is MONITORING ONLY". The refusal is the end state; no redistribution spec is created** |
| Q-2 | Is the native enum error ("invalid value for route-action ... expected one of monitor") enough, or must the refusal say why `redistribute` is gone (approach B, a custom validator with its own message)? | Native error is enough: it names the leaf, the value, and what is allowed, and the guide states the reason | B adds a central validator registration and keeps a refused value in completion. **Answered by the owner, 2026-10-13: no approach B; error messages must be user friendly and "propose what is known is valid from the yang".** The generic enum refusal (today `invalid enum: %q (expected one of: %s)`, `config/schema.go` and `config/parser_list.go`) is made friendly for every YANG enum in a separate spec, plan/immediate/spec-config-enum-refusal-proposes-valid-values.md; this spec relies on that generic message and adds no BMP-specific text |
| Q-3 | The two existing `processRouteMonitoring` tests in `route_action_test.go` assert only "does not panic". Journal row only (recommended, out of this goal), or strengthen them here? | Strengthen here | **Answered by the owner, 2026-10-13: "Yes", strengthen here.** `TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB` is wrong about what it asserts (its nil plugin returns before the store, so it passes with storing removed) and the monitor path is what AC-5 relies on, so `ai/rules/rule-precedence.md` fixes it on the spot. See AC-9 |

No owner approval is recorded as given. Status stays `design` until the owner answers Q-1 and Q-2 and approves the design.

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- BMP redistribution is not built. Whether a spec for it exists depends on Q-1; if the owner answers yes, that spec is created in `plan/` and named here.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
