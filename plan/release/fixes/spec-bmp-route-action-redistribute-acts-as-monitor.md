# Spec: bmp-route-action-redistribute-acts-as-monitor

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P1 |
| Release reason | Accepted receiver action must not silently perform monitoring instead of redistribution. |
| Scope | protocol |
| Depends | Satisfied: `spec-config-enum-refusal-proposes-valid-values` closed in 13310151fb and 9b4f33044a, with its multi-surface enum-refusal proofs. |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Closure outcome (2026-11-13).** Approach A is implemented in fa74dc4c00 and
fb0f9181ec: the receiver accepts `monitor` alone and refuses `redistribute`.
The Task, Current Behavior and design records below describe the research
baseline. The closure audit records the current source and proof. Owner Q-1
settles the goal: "NO BMP is MONITORING ONLY" (2026-10-13).

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
`plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` (Peer Down reasons, sFlow
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
`plan/post-release/features/spec-vrf.md` records for `vrf`. Implementing it is the larger of the two:
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

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec |
|-----------|------------------------------------------|--------------------|-----------|
| Native YANG enumeration refusal | `internal/component/config/schema.go` `ValidateLeafValue` (enum arm), `internal/component/config/yang/validator.go` `validateEnumeration` | Refuses a leaf value outside its `enumeration` on file parse, `set` parse and tree validation, naming the leaf and the allowed values | Extends: deleting `enum redistribute` makes this mechanism the refusal, with no new code |
| Custom `ze:validate` validators | `internal/component/config/validators.go` `unimplementedVRFValidator`, registered in `validators_register.go` | Refuses a whole leaf Ze does not honor | Does not apply: approach B rejected by owner Q-2; a central registration is not needed when the enum already refuses |
| Protocol-namespaced RIB injection | `internal/component/bgp/plugins/rib/rib_inject.go` `handleInjectWireRoute`; candidates from `bgpPeers` only (`rib_commands.go` `gatherPrefixCandidatesLocked`, `gatherKeyCandidatesLocked`) | Stores BMP Route Monitoring routes in `ribInPool[bmp]`, outside best-path | Does not change: it is the `monitor` behavior the surviving value names |
| Receiver config reconcile | `internal/component/bgp/plugins/bmp/receiver.go` `applyReceiverConfig` | Reconciles listeners and stores `max-sessions`; today also logs the redistribute warning | Extends by deletion: the warning branch and `receiverConfig.RouteAction` go, the reconcile stays |
| Enum agreement test helper | `internal/component/bgp/plugins/bmp/yang_enum_agreement_test.go` `yangEnumValues` | Reads an enumeration's values from the loaded `ze-bmp-conf` module | Extends: its path walk becomes a leaf-entry accessor the new AC-6 test also uses for the default |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config text `environment { bmp { route-action <value>; } }`, from a config file at daemon start, `ze config validate`, the CLI editor `set`, or a reload.
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
| A-1 | BMP redistribution is unwanted; monitoring is the complete receiver goal. | Owner Q-1, 2026-10-13: "NO BMP is MONITORING ONLY". | A different owner design would be required. | Q-1 and the approved approach A. | confirmed |
| A-2a | Out-of-enum values are refused by validate, startup and reload. | `leaf_value.go` `ValidateLeafValue` / `validateLeafRestrictions`; YANG `validateEnumeration`. | Unsupported intent could silently reach the plugin. | Owned parse/start and reload tests: recorded YANG-revert RED, restored GREEN and 80 load invocations each. | confirmed |
| A-2b | Generic enum refusal covers editor, web, REST and gNMI config paths. | Closed enum-refusal parent and its final multi-surface audit. | A caller could bypass the config restriction. | Parent A 13310151fb / B 9b4f33044a, with committed unit, live and load proofs. This is the cited generic proof, not new BMP-specific transport proof. | confirmed |
| A-3 | No intended-valid repository config requests receiver redistribution. | Tracked `route-action` search, with the FRR scenario as positive control. | An existing valid user would need conversion. | Only the FRR receiver scenario explicitly selects monitor. New refusal tests intentionally hold invalid redistribute input; published-site files are snapshot test inputs. | confirmed |
| A-4 | Published site fixtures are snapshot inputs, independent of live YANG. | `internal/le/site/config_test.go` `configurationPaths` copies `published-yang-config-tree.json`. | The published-mirror assertion could change. | Main ran `./le --name session test unit pkg path ./internal/le/site/ run '^TestTheConfigurationMirrorReadsAsThePublishedMirror$'`: package PASS, 125.826s, job-unit-pkg-77898dc0.log. | confirmed |
| A-5 | The removed RouteAction field had no surviving runtime reader outside BMP. | Current `receiverConfig` and `applyReceiverConfig`; historical field/reference search. | Deletion could break another consumer. | Tracked BMP `RouteAction` search now matches only the singleton-enum test name. The approved vacuous-field-test deletion landed in fa74dc4c00. | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator's stored config holding `route-action redistribute` stops loading after upgrade, so the daemon does not start | parse error naming `route-action` at start | Ze is pre-release, so no stored config is owed a migration (`ai/rules/pre-release.md`); the error names the leaf and the allowed value, and the guide says to delete the line or write `monitor` |
| R-2 | The enum refusal names `monitor` as the accepted value but not WHY `redistribute` is gone, which `ai/rules/user-facing-errors.md` may judge insufficient | review of the AC-1 message | Settled by owner Q-2 (2026-10-13): no approach B. The generic enum refusal is made friendly for every YANG enum by `spec-config-enum-refusal-proposes-valid-values`, and the guide states the reason. The two specs' code may land in either order (closure of this spec still waits on that spec, for A-2b), because AC-1 pins only words both message texts carry (`route-action`, `redistribute`, `monitor`), which is also that spec's R-7 |
| R-3 | A one-value enumeration reads as pointless and a later cleanup deletes the leaf, which (per the `unimplementedVRFValidator` note) would make `route-action redistribute` load SILENTLY again | a diff that removes the leaf | The YANG `description` states why the leaf stays with one value; the AC-1 `.ci` goes red if the leaf is removed and the value is then silently accepted (it asserts exit 1) |
| R-4 | Deleting the warning branch and field leaves a comment elsewhere describing the warning | grep for "not yet implemented, using monitor" | Phase 2 greps and removes every occurrence (`ai/rules/stale-comments.md`) |
| R-5 | Reload with the value refused leaves the old config running, and the operator reads that as success | reload returns an error | AC-3 asserts the reload is refused with the message, the daemon keeps running, and the test peer's session stays up. The listener set is not asserted: the receiver's "listening" line is logged at Info and the default level is WARN (see AC-2), and the reload is refused at parse, before `applyReceiverConfig` runs |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A config holding `route-action redistribute` is rejected (intended); if the refusal leaked to `monitor` or to the absent leaf, every BMP receiver config would be rejected and the receiver would not start |
| How is it reverted? | Single commit revert; no wire, storage, or config-format migration |
| Who else touches this path? | `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` (sender side, `bmp_events.go`, no shared file); the BMP sender YANG block in the same `ze-bmp-conf.yang` file (different container, `bgp/bmp/sender`) |

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
| SSH editor set, web form, REST session commit, gNMI `Set` with an out-of-enum value (A-2b) | → | `ValidateValueAtPath`, the web form value check, `ValidateContent` enum refusal (generic over every YANG enum, not BMP-specific) | cited, not owned here, validated when `spec-config-enum-refusal-proposes-valid-values` closes: its AC-6 to AC-9 tests `TestValidateValueAtPathEnumRefusal`, `test/editor/validation/enum-refusal-set.et`, `TestWebFormRefusesEnumAtSet`, `test/web/config-enum-refusal.wb`, `test/plugin/api-config-enum-refusal.ci`, `test/plugin/gnmi-set-enum-refusal.ci` (driven by that spec's `le test fixture` gNMI client, A-6) |
| Route Monitoring message from a monitored router | → | `processRouteMonitoring` → `(*sdk.Plugin).InjectWireRoute` | `TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB` (rewritten, AC-9) |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze config validate -` on a config whose `environment bmp` writes `route-action redistribute;` | exit 1; output contains `route-action`, `redistribute` and `monitor`. Nothing else of the message text is pinned: `spec-config-enum-refusal-proposes-valid-values` replaces the generic enum refusal body, and these three words are in both the current and the replacement text, so the two specs land in either order (that spec's R-7). That `monitor` is the only accepted value is proven by AC-6, not by this message. Proof: `test/parse/bmp-route-action-refuses-redistribute.ci`, first command |
| AC-2 | The same config as the daemon's startup config (`ze -`) | daemon exits non-zero, and stderr carries the AC-1 words. No "bmp: receiver listening" reject is asserted: that line is logged at Info (`applyReceiverConfig` path in `receiver.go`) and `slogutil.Logger` defaults to WARN, so it never prints even when a listener starts, and a reject on it would be vacuous. Exit code plus refusal text is the evidence, the shape of `test/parse/tacacs-key-required.ci`. Proof: same `.ci`, second command |
| AC-3 | Daemon running a valid BMP receiver config, then SIGHUP to a config that adds `route-action redistribute;` | reload refused with the AC-1 words on stderr; the daemon keeps running and the BGP peer session in the test stays up (exit 0 after SIGTERM). The listener set is not asserted (see R-5). Proof: `test/reload/bmp-route-action-redistribute-refused.ci` |
| AC-4 | `route-action monitor;` written explicitly, and the leaf absent | both validate (exit 0). Proof: AC-1 `.ci` accepted counterparts; `test/parse/bmp-config.ci` unchanged |
| AC-5 | A Route Monitoring message for a prefix also learned from a real BGP peer, and for a prefix only BMP knows, under default config | BMP routes appear in `show bmp rib`, never as a best-path candidate, unchanged. Proof: `test/plugin/bmp-lg-bestpath-isolation.ci` and `TestBMPRoutesExcludedFromBestPath` pass unchanged |
| AC-6 | The `ze-bmp-conf` schema at `environment/bmp/route-action` | the enumeration is exactly `[monitor]` and the leaf default is `monitor`. Editor completion is derived from the enumeration, so it follows, but the test reads the schema, not completion. Proof: `TestRouteActionEnumIsMonitorOnly`, which reads the values through `yangEnumValues` and the default through goyang's `(*yang.Entry).SingleDefaultValue` on the same leaf entry. `yangEnumValues` returns the values only and never exposes the entry, so its path walk is factored into a leaf-entry accessor in `yang_enum_agreement_test.go` that `yangEnumValues` then reuses, and the new test calls that accessor for the default |
| AC-7 | Source after the change | the string "route-action redistribute is not yet implemented" appears nowhere; `receiverConfig` has no `RouteAction` field; the YANG `ze:help` and `description` and `docs/guide/bmp.md` no longer say `redistribute` exists or is planned. Proof: grep in the Deliverables table, plus the build |
| AC-8 | Interop scenario `bmp-locrib-receiver-frr` (`ze.conf` writes `route-action monitor;`) | still loads and passes. Proof: `./le test integration` scenario run, recorded in the Goal Validation table |
| AC-9 | A Route Monitoring message reaches `processRouteMonitoring` (owner answer Q-3) | with a recording stand-in for the RIB injection, a well-formed UPDATE is injected exactly once with protocol `bmp` (`protocolBMP`), the peer key the composite key `bmpCompositeKey` builds from the remote address and the message's per-peer header, and a body equal to the bytes of the message's BGP UPDATE after the 19-byte header (`bgpHeaderSize`); an UPDATE shorter than 19 bytes injects nothing. The seam: both current tests leave the plugin's SDK handle unset, and `processRouteMonitoring` returns on an unset handle before the length guard, so neither reaches either branch. The handle is a concrete `*sdk.Plugin`. The test builds a direct bridge with `rpc.NewDirectBridge`, sets its inject handler to a recorder with `SetInjectWireRoute`, calls `SetReady`, wraps one end of a pipe with `rpc.NewBridgedConn` around that bridge, and passes the result to `sdk.NewWithConn`. `(*sdk.Plugin).InjectWireRoute` then takes the bridge path, because `HasInjectWireRoute` holds once the bridge is ready and the handler is set. Proof: `TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB` and `TestProcessRouteMonitoring_ShortUpdate_Skipped` rewritten to assert both, each shown red with the injection call removed (first) or the length guard removed (second) |

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
| `TestReceiverConfig_RouteActionDefault` (DELETE, owner Q-4 "ok", 2026-10-13) | `internal/component/bgp/plugins/bmp/route_action_test.go` | The removed test asserted only a Go zero value. `TestRouteActionEnumIsMonitorOnly` proves the actual YANG default. Independent judge approval and deletion are retained in fa74dc4c00's `Weakening-approved:` trailer. | Done |
| `TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB`, `TestProcessRouteMonitoring_ShortUpdate_Skipped` (REWRITE, owner Q-3) | `internal/component/bgp/plugins/bmp/route_action_test.go` | rewritten per AC-9 through the bridged-conn seam: the first asserts one injection with protocol `bmp`, the peer key `bmpCompositeKey` builds from the remote address and the per-peer header, and the UPDATE bytes after the 19-byte header (`bgpHeaderSize`) as body; the second asserts no injection for an UPDATE shorter than 19 bytes. Red: the first with the `InjectWireRoute` call removed, the second with the length guard removed. No journal row: owner Q-3 says strengthen here. Both are listed today in `docs/features/test-health.md` under "no reachable failure call"; that page is generated and follows by `./le test health update`, with no hand edit | |

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
| `bmp-route-action-refuses-redistribute` (new) | `test/parse/bmp-route-action-refuses-redistribute.ci` | shape of `test/parse/asn4-refuses-boolean-spelling.ci`: stdin `redistribute` → `ze config validate -` exit 1 with `route-action`, `redistribute` and `monitor` as the only pinned words (no `:` in needles, no other message text, so it survives the enum-refusal spec); stdin `monitor` and stdin with the leaf absent → exit 0; then `ze -` on the `redistribute` stdin exits non-zero with the same three words. No reject on "receiver listening" (Info line, default level WARN, so the reject would be vacuous; AC-2). Written and iterated in `test/draft/parse/`, promoted to `test/parse/` when green (AC-1, AC-2, AC-4) | |
| `bmp-route-action-redistribute-refused` (new) | `test/reload/bmp-route-action-redistribute-refused.ci` (written and iterated in `test/draft/reload/`, then promoted) | shape of `test/reload/config-reload-invalid-validator.ci`: daemon up with a BMP receiver and one test peer; `action=rewrite` to a config adding `route-action redistribute;`, `action=sighup`, then `sigterm`; stderr carries the refusal, the peer saw no NOTIFICATION, exit 0 (AC-3) | |
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
- `internal/component/bgp/plugins/bmp/route_action_test.go` - rewrite `TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB` and `TestProcessRouteMonitoring_ShortUpdate_Skipped` per AC-9 through the bridged-conn seam: a direct bridge from `rpc.NewDirectBridge` with its inject handler set to a recorder by `SetInjectWireRoute` and `SetReady` called, wrapped by `rpc.NewBridgedConn` around one end of a pipe and handed to `sdk.NewWithConn`; delete `TestReceiverConfig_RouteActionDefault` (owner Q-4 "ok", 2026-10-13), with prior independent test-judge approval, retained in fa74dc4c00's native trailer
- `internal/component/bgp/plugins/bmp/yang_enum_agreement_test.go` - factor the path walk out of `yangEnumValues` into a leaf-entry accessor that returns the goyang entry, make `yangEnumValues` reuse it, and add `TestRouteActionEnumIsMonitorOnly`, which reads the values through `yangEnumValues` and the default through the accessor and `SingleDefaultValue` (AC-6)
- `docs/guide/bmp.md` - receiver field table row and the Looking Glass Integration paragraph (`// Design:` header of `receiver.go` names this page)
- `plan/journal/unwired-feature.md` - the 2026-09-04 row naming `bmp/route-action` records the fix (append, do not rewrite the row's history)

## Files to Create
- `test/parse/bmp-route-action-refuses-redistribute.ci` - AC-1, AC-2, AC-4; drafted in `test/draft/parse/` and promoted when green (`ai/rules/testing.md`: a `.ci` is written and iterated in `test/draft/<suite>/`)
- `test/reload/bmp-route-action-redistribute-refused.ci` - AC-3; drafted in `test/draft/reload/` and promoted when green

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

**Generated surfaces, no hand edit.** The published site carries the stale text in the `gh-pages` checkout (`guides/bmp/index.md` field table and the "controls future behavior" list, `data/yang-config-tree.json`, `llms-full.txt`). All are generated from `docs/guide/` and the YANG, so they follow by the site publish. `docs/features/test-health.md` lists both AC-9 tests under "no reachable failure call"; it is generated and follows by `./le test health update`. Neither is edited by hand in this spec.

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

Steps follow approach A, which owner answers Q-1 (monitoring only, the refusal is the end state) and Q-2 (no approach B) settle. Owner answer Q-4 ("ok", 2026-10-13, commit f8c0c10c3b) allows the deletion in Phase 3.

1. **Phase: Wiring (MANDATORY FIRST)** -- the failing tests through the real entry points
   - Tests: `bmp-route-action-refuses-redistribute.ci`, `bmp-route-action-redistribute-refused.ci`, `TestRouteActionEnumIsMonitorOnly`, and the two AC-9 rewrites
   - Files: the two `.ci` written in `test/draft/parse/` and `test/draft/reload/` and iterated there, `yang_enum_agreement_test.go`, `route_action_test.go` (AC-9 rewrites); re-run the A-3 grep
   - Verify: the three schema tests red against HEAD (validate exits 0, reload accepted, enum has two values); the AC-9 rewrites green at HEAD and shown red under their two mutations (injection call removed, length guard removed); paste output. Promote the two `.ci` to `test/parse/` and `test/reload/` once green in Phase 2
2. **Phase: Schema** -- delete `enum redistribute`, rewrite the leaf `description`, edit `docs/guide/bmp.md` in the same phase
   - Tests: the three above go green; `test/parse/bmp-config.ci`, `test/plugin/bmp-lg-bestpath-isolation.ci`, `test/plugin/bmp-lg-ingest.ci` stay green; scoped `internal/le/site` package test stays green (A-4)
   - Files: `ze-bmp-conf.yang`, `docs/guide/bmp.md`
3. **Phase: Dead code** -- delete the warning branch, `receiverConfig.RouteAction`, and `TestReceiverConfig_RouteActionDefault` (owner Q-4 "ok", 2026-10-13), with prior independent test-judge approval, retained in fa74dc4c00
   - Tests: scoped `internal/component/bgp/plugins/bmp` package test green; `gopls references` on the field empty before deletion (A-5); grep for the warning text empty (R-4)
   - Files: `receiver.go`, `bmp.go`, `route_action_test.go`
4. **Phase: Proof** -- revert walk on the YANG edit (rebuild `ze`, both `.ci` red, restore, green, record); run `bmp-locrib-receiver-frr` (AC-8); append the fix to the `plan/journal/unwired-feature.md` row

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

-> Decision (2026-11-11, proof run): every deliverable above is met.
- Code, schema and guide: commit fa74dc4c00. The two greps return only unrelated mentions (the redistribute orchestrator, a test comment, and the guide sentence that states the refusal); the warning text is gone.
- Refusal: `test/parse/bmp-route-action-refuses-redistribute.ci` and `test/reload/bmp-route-action-redistribute-refused.ci` went red with `enum redistribute` restored (validate exit 0, reload accepted) and green with it removed; each passed 80 invocations under `./le test stress-repro` load; promoted in fb0f9181ec (the parse test's daemon-start deadline raised from 10s to 30s, as other parse tests use).
- Unit: `TestRouteActionEnumIsMonitorOnly` and both `TestProcessRouteMonitoring` tests pass (`go test -tags ze_bgp,ze_bmp`).
- `monitor` unchanged: `bmp-lg-bestpath-isolation`, `bmp-lg-ingest`, `bmp-lg-disconnect`, `bmp-lg-http-routes` and `bmp-config` pass; AC-8 `bmp-locrib-receiver-frr` against FRR passes (`./le test integration interop`, 1 passed, 0 failed).
- The closure-only dependency is satisfied: `spec-config-enum-refusal-proposes-valid-values` closed in 13310151fb and 9b4f33044a on 2026-11-13. Its committed multi-surface proofs resolve A-2b.

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | the refusal holds on every config path so no path can enable a value the plugin silently ignores. This spec proves validate, start and reload (A-2a); editor set, web, REST and gNMI are proven by the enum spec's AC-6 to AC-9 tests, cited in A-2b and validated when that spec closes; `environment/bmp` has no env-var route |
| Fail-open | No forwarding bypass is claimed fixed: the old value already only monitored. The enum now refuses unsupported intent, while `processRouteMonitoring` still injects only `protocolBMP` and `handleInjectWireRoute` stores outside BGP candidates. |

### Closure security review

| Concern | Producer evidence and disposition |
|---------|-----------------------------------|
| Validation and authorization | The singleton YANG enum is the authority before plugin config delivery. Parse/start/reload assertions include accepted monitor/absent controls. Parent closure proves generic editor, web, REST, gRPC and gNMI paths. No env override or new authorization surface is added. |
| Injection, path traversal and output leakage | No command, file path, query, format string or secret handling is added. The shared refusal and fix rendering are the closed parent's work. The receiver field/warning are deleted, with no substitute execution path. |
| Bounds, allocation and denial of service | No runtime allocation, loop, queue, retry, wire parser or numeric input changes. Test recorder allocations depend only on fixed test bodies. Listener reconcile and session cap remain unchanged. |
| Concurrency and lifecycle | The removed branch had no shared-state effects. Existing receiver locking, listener cleanup and atomic session cap remain. Refusal occurs before reload apply, so plugin crash/restart, shutdown/drain, handshake and startup-dispatch scenarios cannot be newly reached by this change. |
| Cryptography and privileges | No key, random source, cipher, socket privilege or dependency changes. BMP transport remains unauthenticated TCP. This fix does not claim transport authentication or a security-vulnerability repair. |
| Protocol boundary | RFC 7854 Section 3.2: "No BMP message is ever sent from the monitoring station to the monitored router." No BMP/BGP bytes or protocol state transitions change here. The existing FRR AC-8 proof is retained. |

Security review found no product defect in this change. No security fix or
follow-up test run is required by step 3 of `/ze-close`.

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
| A (recommended): remove `enum redistribute` from the YANG; native enum refusal; delete warning and field | B: keep the value and refuse it with a `ze:validate` custom validator (the `unimplementedVRFValidator` shape). C: implement redistribution. D: delete the whole leaf | A is the minimum change and adds no central entry: B needs a validator registered in `internal/component/config/validators_register.go` (a central list) and keeps completing a value Ze refuses; its one gain is a refusal message that says WHY, and owner Q-2 (2026-10-13) rejected it: the friendly message comes from the generic enum-refusal spec for every enum. C is a feature, not a fix: RFC 7854 defines no station-side route use, Section 11 says an unauthenticated feed can "trick a monitoring station into accepting false information", and BMP Route Monitoring carries another router's pre-policy Adj-RIB-In with next hops that belong to that router's peers; a trust, next-hop and loop-prevention design is owed before any code (Q-1). D looks simpler but, per the `unimplementedVRFValidator` note, an unknown leaf is skipped by `ValidateTreeAllModules`, so `route-action redistribute` could load silently again (the parser's handling of an unknown leaf was not verified here, and A does not depend on it) |

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
| Q-2 | Is the native enum error ("invalid value for route-action ... expected one of monitor") enough, or must the refusal say why `redistribute` is gone (approach B, a custom validator with its own message)? | Native error is enough: it names the leaf, the value, and what is allowed, and the guide states the reason | B adds a central validator registration and keeps a refused value in completion. **Answered by the owner, 2026-10-13: no approach B; error messages must be user friendly and "propose what is known is valid from the yang".** The generic enum refusal (today `invalid enum: %q (expected one of: %s)`, `config/schema.go` and `config/parser_list.go`) is made friendly for every YANG enum in a separate spec, spec-config-enum-refusal-proposes-valid-values; this spec relies on that generic message and adds no BMP-specific text |
| Q-3 | The two existing `processRouteMonitoring` tests in `route_action_test.go` assert only "does not panic". Journal row only (recommended, out of this goal), or strengthen them here? | Strengthen here | **Answered by the owner, 2026-10-13: "Yes", strengthen here.** `TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB` is wrong about what it asserts (its nil plugin returns before the store, so it passes with storing removed) and the monitor path is what AC-5 relies on, so `ai/rules/rule-precedence.md` fixes it on the spot. See AC-9 |
| Q-4 | May the vacuous `TestReceiverConfig_RouteActionDefault` be deleted when its field is removed and the YANG default has a real test? | Delete it | **Owner answer, 2026-10-13 (f8c0c10c3b): "ok".** Deletion landed in fa74dc4c00 after independent `ze-test-judge` approval. The commit's `Weakening-approved:` trailer retains the verdict. The old unit established no schema-default behavior. |

Q-1 to Q-4 are answered (2026-10-13). Round 3 of the design review is clean.

**Owner decisions, 2026-11-11 (Thomas: "do it").**
- The design (approach A) is approved, so Status moved `design` to `ready` and the spec was then claimed for implementation.
- One-off WIP cap exception: this one spec may be claimed above the WIP cap. It was claimed with `ZE_SPEC_WIP_CAP=19` when 18 specs were in progress. This is a permission for this piece of work, not a rule, and is recorded here only.

**Q-4 route correction (2026-11-11).** The historical design named an obsolete `test/weakened/` ledger. The edit hook required independent `ze-test-judge` approval through `./le test weakened approve`, recorded in the main session's native approval file. fa74dc4c00 carries the approved deletion and its `Weakening-approved:` trailer. Owner Q-4 settled scope; the independent verdict settled the test change.

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- None. Owner Q-1 (2026-10-13): BMP is monitoring only, so BMP redistribution is not a deferred feature and no spec for it is created.

## Design Review Record

### Design review history

| Round | Date | Result | Disposition |
|-------|------|--------|-------------|
| 1 | 2026-10-13 | 0 BLOCKER, 6 ISSUE, 4 NIT (review at HEAD 5026ce3649) | Fixed in this edit: I1 AC-1 pins only `route-action`, `redistribute`, `monitor`, so either landing order with the enum-refusal spec holds (AC-1, Functional row 1, R-2); I2 TDD row 3 is AC-9's rewrite, the weak-test journal row is dropped from it and Phase 4, Known Limitations and the approval line restated from Q-1 to Q-3, R-2 and the B rationale cite Q-2; I3 AC-9 names the bridged-conn seam and the exact protocol, peerKey and body, and Files to Modify carries the two rewrites; I4 the vacuous "receiver listening" reject is dropped (Info line under a WARN default) in AC-2 and Functional row 1; I6 A-2, the Wiring table and the Security row claim only validate, start and reload here and cite the enum spec's AC-6 to AC-10 tests for editor, web, REST and gNMI; N7 both `.ci` drafted in `test/draft/<suite>/`; N8 generated site and test-health surfaces noted, no hand edit; N9 AC-6 reads the enumeration and names `SingleDefaultValue` for the default; N10 R-5 no longer claims a listener-set assertion. I5 is NOT fixed: the deletion of `TestReceiverConfig_RouteActionDefault` is owner question Q-4, open |
| 2 | 2026-10-13 | 0 BLOCKER, 3 ISSUE, 2 NIT (review at HEAD 8e1595acf2) | Fixed in this edit: I1 A-2 split into A-2a (validate, start, reload; this spec's `.ci`) and A-2b (editor set, web, REST, gNMI; validated when the enum-refusal spec closes), metadata Depends names that spec for closure only, the Goal Gate A-N row and R-2 say closure waits on it, Wiring row 5 and the Security row cite A-2b; I2 "and commit" dropped from the editor wiring row, because no test at HEAD or in the enum spec commits a bad non-peer enum through the editor; I3 AC-9, TDD row 3 and Files to Modify rewritten as prose naming `rpc.NewDirectBridge`, `SetInjectWireRoute`, `SetReady`, `rpc.NewBridgedConn`, `sdk.NewWithConn` and "the bytes after the 19-byte header (`bgpHeaderSize`)", with no Go statements; N4 AC-6 and Files to Modify name the leaf-entry accessor that `yangEnumValues` reuses; N5 the A-2b citation names `test/plugin/api-config-enum-refusal.ci`, `test/plugin/gnmi-set-enum-refusal.ci` (fallback `internal/component/gnmi/integration_test.go`, enum spec A-6) and `test/web/config-enum-refusal.wb` |
| 3 | 2026-10-13 | 0 BLOCKER, 0 ISSUE, 2 NIT (review at HEAD fd25b8e310, `review-bmp-route-action-r3.md`) | Fixed in this edit: N1 gNMI citation names the enum spec's settled `le test fixture` client (no integration_test.go fallback, after 2185e61560); N2 Data Flow entry point says editor `set`, not commit. Design review complete; open: owner Q-4 (delete `TestReceiverConfig_RouteActionDefault`), then owner approval of the design |
| 4 | 2026-10-14 | Q-4 recorded as answered (f8c0c10c3b) | Owner answer "ok" (2026-10-13) applied to the Unit Tests delete row, Files to Modify, the Implementation Steps preamble and Phase 3, and the Owner Questions summary. Open: owner approval of the design |

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
- [ ] Scoped tests of the affected behavior pass in the main checkout. No agent runs any `./le verify` command or develops/tests in a worktree. GitHub CI verifies a pushed commit.
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`. A-2b is confirmed only by the closed `spec-config-enum-refusal-proposes-valid-values` and its named tests green, so this spec does not close while that spec is open (metadata Depends, closure only)
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Route any new lesson to its governing surface. Closure requires no separate learned summary.
- [ ] **Commit A:** edited spec, closure documentation, news and journal evidence, through `./le commit create`.
- [ ] **Commit B:** remove the spec through the same generated script, after A preserves it in history. Leave the file in place until the script runs.

## Implementation Summary

### What Was Implemented

The receiver YANG retains `monitor`, its default, and removes `redistribute`.
The unused `receiverConfig.RouteAction` field and warning in
`BMPPlugin.applyReceiverConfig` are gone. The data path is unchanged.

### Bugs Found/Fixed

The accepted value previously selected behavior Ze did not implement.
The parse/start and reload tests now reject it and retain accepted counterparts.
The two `processRouteMonitoring` tests now assert the actual SDK injection
instead of returning early through a nil plugin handle.

### Documentation Updates

`docs/guide/bmp.md` receiver field table and Looking Glass Integration now
state monitor-only behavior and the valid replacement. fa74dc4c00 carries the
behavior update. Closure adds the field table's immediate source anchor and
the canonical operator news fragment. Main owns the required doc check.

### Deviations from Plan

Q-4's historical weakening-ledger instructions were obsolete. The independent
judge approval used the native approval route, and fa74dc4c00 carries both
`Weakening-approved:` trailers. No behavior or coverage was removed to clear a red.

### Risks and observations

| Risk | Closure disposition |
|------|---------------------|
| R-1, a saved redistribute config no longer loads. | Intended refusal. Remove the setting or select monitor. Ze is pre-release and no migration is required. |
| R-2, an unclear refusal. | Resolved by the closed generic enum-refusal work; current tests assert the full refusal and exact valid fix. |
| R-3, a later cleanup deletes the one-value leaf. | The schema description explains why the leaf remains. Entry-point refusal and singleton/default tests discriminate its removal or widening. |
| R-4, stale warning references. | Current runtime source has no warning or field. Historical design/commit records remain evidence of the old defect. |
| R-5, a refused reload appears successful. | The reload emits the refusal and retains the running daemon. The test asserts this sequence; it does not assert the listener set. |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The spec named an old weakening ledger. | Native independent-judge approval is required. | Edit-hook refusal recorded in the 2026-11-11 Q-4 correction. | Approval trailers landed in fa74dc4c00. |
| approach | The record retained worktree verification and a mandatory learned summary. | Main-checkout scoped proof and lesson routing are the current rules. | Closure rule review. | Corrected the checklist, with no prohibited verification run. |
| approach | The promised BMP journal annotation was absent. | The 2026-09-04 row still included the BMP symptom without a fix record. | Closure read of `plan/journal/unwired-feature.md`. | Added a separate concise fixed BMP row, preserving the original row's other open surfaces. |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Refuse the unsupported receiver action. | Done | `ze-bmp-conf.yang` `route-action`. | Exactly `monitor`, default `monitor`; parse/start and reload RED-to-GREEN proof in fb0f9181ec. |
| Preserve monitoring without best-path or FIB input. | Done | `BMPPlugin.processRouteMonitoring`, `RIBManager.handleInjectWireRoute`. | BMP namespace remains separate. Preserved ingest/isolation and FRR proof recorded in Deliverables. |
| Remove false promises and unreachable warning code. | Done | `receiverConfig`, `BMPPlugin.applyReceiverConfig`, `docs/guide/bmp.md`. | fa74dc4c00 deletes the field and warning and corrects the guide. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `bmp-route-action-refuses-redistribute.ci`, validate step. | Exit 1, full path, rejected value, accepted value and exact fix are asserted. |
| AC-2 | Done | Same parse `.ci`, daemon-start step. | Exit 1 with refusal, no vacuous listener-log assertion. |
| AC-3 | Done | `bmp-route-action-redistribute-refused.ci`. | After BGP UPDATE and EOR, rewrite/SIGHUP must emit the refusal and accept SIGTERM with exit 0. |
| AC-4 | Done | Parse `.ci` monitor and absent cases; `bmp-config.ci`. | Both accepted counterparts remain and passed. |
| AC-5 | Done | `TestBMPRoutesExcludedFromBestPath`, `bmp-lg-bestpath-isolation.ci`. | Actual namespace storage and BGP-only candidate selection. Preserved proof recorded in Deliverables. |
| AC-6 | Done | `TestRouteActionEnumIsMonitorOnly`, `yangLeafEntry`, `yangEnumValues`. | Exact singleton enumeration and `SingleDefaultValue` asserted from embedded YANG. |
| AC-7 | Done | `receiverConfig`, `applyReceiverConfig`, receiver schema and guide. | Source grep finds no warning or runtime field. The guide mentions `redistribute` only to explain its refusal. |
| AC-8 | Done | `bmp-locrib-receiver-frr` scenario. | Recorded native integration run: 1 passed, 0 failed, with explicit `route-action monitor`. |
| AC-9 | Done | Both `TestProcessRouteMonitoring` tests via `bridgedBMPPlugin`. | Exact one-call protocol/key/body and zero calls for a short UPDATE. This recorder proves forwarding, not RIB parsing or storage. Valid route ingestion has separate preserved controls. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRouteActionEnumIsMonitorOnly` | Done | `bmp/yang_enum_agreement_test.go`. | Singleton enumeration/default asserted, recorded green. |
| Delete `TestReceiverConfig_RouteActionDefault` | Done | `bmp/route_action_test.go`, fa74dc4c00. | Owner Q-4 and independent judge approval retained in commit trailer. It proved only a removed field's zero value. |
| `TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB` | Done | `bmp/route_action_test.go`. | Ready bridge recorder asserts exact injection. Recorded green and injection-removed RED control. |
| `TestProcessRouteMonitoring_ShortUpdate_Skipped` | Done | `bmp/route_action_test.go`. | Set SDK handle reaches the real length guard. Recorded green and guard-removed RED control. |
| `bmp-route-action-refuses-redistribute` | Done | `test/parse/`. | Promoted in fb0f9181ec after YANG revert RED, restored GREEN and 80 load invocations. |
| `bmp-route-action-redistribute-refused` | Done | `test/reload/`. | Promoted in fb0f9181ec after YANG revert RED, restored GREEN and 80 load invocations. |
| Preserved config, ingest, isolation, disconnect and HTTP route tests | Done | `bmp-config.ci`, `bmp-lg-ingest.ci`, `bmp-lg-bestpath-isolation.ci`, `bmp-lg-disconnect.ci`, `bmp-lg-http-routes.ci`. | Recorded passing Deliverables proof. No new claim is inferred from their names. |
| `bmp-locrib-receiver-frr` | Done | Existing interop scenario. | FRR receiver integration result recorded above. No protocol bytes changed. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` | Done | Enum removal and rationale in fa74dc4c00. |
| `internal/component/bgp/plugins/bmp/receiver.go` | Done | Warning removal in fa74dc4c00. Listener reconcile and session cap unchanged. |
| `internal/component/bgp/plugins/bmp/bmp.go` | Done | Runtime field removal and monitor-only comment in fa74dc4c00. |
| `internal/component/bgp/plugins/bmp/route_action_test.go` | Done | Bridge assertions and approved vacuous-test deletion in fa74dc4c00. |
| `internal/component/bgp/plugins/bmp/yang_enum_agreement_test.go` | Done | Shared leaf accessor and singleton/default test in fa74dc4c00. Old helper checks survive. |
| `docs/guide/bmp.md` | Done | Operator behavior corrected in fa74dc4c00. Immediate field-table source anchor added at closure. |
| `plan/journal/unwired-feature.md` | Done | Closure adds a fixed BMP addendum. The multi-surface historical row stays open for its other occurrences. |
| `test/parse/bmp-route-action-refuses-redistribute.ci` | Done | Promoted in fb0f9181ec, later strengthened with the shared friendly refusal. |
| `test/reload/bmp-route-action-redistribute-refused.ci` | Done | Promoted in fb0f9181ec, later strengthened with the shared friendly refusal and correct corpus diagnostic. |

### Audit Summary

29 planned items: 29 Done, 0 Partial, 0 Skipped, 0 scope changes.
Source review is CLEAN. The native hash check passed, and the final
documentation findings are dispositioned below. The schema-tightening
corpus obligation is satisfied: main ran `./le --name session test unit pkg path
./internal/component/config/cli/ run '^TestRepositoryConfigsValidate$'` and the
package passed in 153.598s, with no skips or failures reported. The named
selector, native test-run handling and actual package result are retained in
`job-unit-pkg-edd5d381.log`; this is scoped proof, not a whole-tree green claim.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Ze refuses an action it does not honor. | Functional and discrimination | Both promoted `.ci` files went RED with the deleted enum restored, then GREEN, each over 80 load invocations. fb0f9181ec retains the result and reason. |
| Monitoring remains visible and isolated. | Functional, unit and interop | Preserved ingest/isolation/disconnect/HTTP/config tests pass. `TestBMPRoutesExcludedFromBestPath` pins the BGP-only candidate. `bmp-locrib-receiver-frr`: 1 passed, 0 failed. |
| No current surface promises receiver redistribution. | Source and documentation | `route-action` declares `monitor` alone. Runtime field and warning are absent. Guide field table and Looking Glass Integration explain monitoring and the valid replacement. Published-site snapshots are fixture inputs, covered by A-4. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None of the agreed BMP work is intentionally omitted. | Owner Q-1 makes monitoring the complete receiver goal. | None. |

## Review Gate

Round 1 scope, declared before the final producer-lens verdict: the complete
BMP changes in fa74dc4c00 and fb0f9181ec, the subsequently strengthened owned
parse/reload assertions, and sibling enum, SDK bridge, RIB-isolation,
functional-fixture and interop-checker producers. The commits carry the
foreign `Spec-work: fib-depth` label, so native history alone is insufficient.
Main pinned the seven owned code files and seven clean control files. The
two `.ci` pins include this closure's comment-only corrections. Foreign SDK,
RIB and fixture edits are excluded from the pinned change set.
Design-review rounds above are not source-review rounds.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bmp-route-action-redistribute-acts-as-monitor-config-enum-refusal-20261112-codex-continue.md`, 14 files, verdict clean, round 1. |
| `./le spec review check` | PASS: `review_gate: OK (13 code files, clean, hashes match)`, over all 14 recorded paths. |
| Rounds | 1. Final source verdict: 0 BLOCKER, 0 ISSUE. Design history is separate. |
| Reviewer lenses used | Logic, wiring and spec completeness; security, boundaries and vacuity; Go style and removed-behavior audit. |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| None | None | Round 1 found no product BLOCKER or ISSUE. | Complete owned change and sibling producers. | Not applicable. |

### Final source verdict

Round 1 is CLEAN. All lenses read producers: logic, wiring and the nine ACs;
security, boundary cases and vacuity; Go style and removed-behavior preservation.
The style pass found no peer-reachable panic, new unbounded runtime loop or
allocation, misleading name, lifecycle gap, invalid-state operation or unused
return dimension. The test-only recorder copies the fixed input to preserve
its observation and closes both pipe endpoints through `t.Cleanup`.

The changed guard has accepted monitor/absent controls and rejecting validate,
startup and reload controls. The owned RED walks prove those guards are reached.
The two bridge units retain their original no-panic property and add exact
injection assertions. Their opaque body does not claim real RIB parsing.
Valid ingestion is proven separately by the preserved fixture and RIB tests.

The existing functional producers `bmpIngest04`, `bmpBestpath04`,
`bmpDisconnect04` and `bmpLGHTTPRoutes04` inspect actual route results and
return failures through `withBMP04`/`Observe`. Candidate producers
`gatherPrefixCandidatesLocked` and `gatherKeyCandidatesLocked` iterate only
`bgpPeers`. The existing FRR checker verifies the session, AS, address and
IPv4 family from the received OPEN. No new interop assertion is authored here.

Record-only NOTEs corrected in this round: old dependency/Q-4/checklist state,
missing immediate table anchor, missing journal addendum, and stale headers in
the two `.ci` files. The comment fixes were drafted, compared with the live
tests with every non-comment line unchanged, then promoted by main. They
change no directive, config, assertion or deadline and owe no runtime rerun.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `test/parse/bmp-route-action-refuses-redistribute.ci` | Yes | `ls` returned this promoted path. Read all commands, configs and assertions. |
| `test/reload/bmp-route-action-redistribute-refused.ci` | Yes | `ls` returned this promoted path. Read all actions and assertions. |
| `test/parse/bmp-config.ci` | Yes | `ls` returned this path and its accepted receiver/sender config was read. |
| `test/plugin/bmp-lg-bestpath-isolation.ci`, `bmp-lg-ingest.ci`, `bmp-lg-disconnect.ci`, `bmp-lg-http-routes.ci` | Yes | `ls` returned all four paths. Registrations and actual fixture assertions were read in `plugin_fixture_04.go` / `plugin_fixture_04_bmp.go`. |
| `test/interop/scenarios/bmp-locrib-receiver-frr/ze.conf` | Yes | `ls` returned this path; it explicitly selects monitor. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Validate refuses the value. | Read parse `.ci`: exit 1 and friendly refusal/fix assertions. Recorded RED-to-GREEN and 80-load proof in fb0f9181ec. |
| AC-2 | Startup refuses the value. | Read same `.ci`: daemon exit 1 and identical refusal assertions, accepted counterparts intact. |
| AC-3 | Reload refuses and preserves the running daemon. | Read reload `.ci`: UPDATE/EOR fence, rewrite/SIGHUP/SIGTERM, refusal plus exit 0. Recorded GREEN and YANG-revert RED in fb0f9181ec. |
| AC-4 | Monitor and omitted leaf remain accepted. | Read parse `.ci` steps 2/3 and `bmp-config.ci`, retained passing proof. |
| AC-5 | BMP never enters best path. | Read `handleInjectWireRoute` protocol namespace and `TestBMPRoutesExcludedFromBestPath` exact one-BGP-candidate assertion, preserved live isolation proof. |
| AC-6 | Enumeration/default are monitor only. | Read `TestRouteActionEnumIsMonitorOnly`: exact `[monitor]`, one default, equal to `monitor`. |
| AC-7 | Dead warning/field and false promise are gone. | Tracked source grep for old warning has no hits. `RouteAction` matches only the surviving schema test name. Guide agrees with current YANG. |
| AC-8 | Existing FRR monitor config still works. | Read scenario `ze.conf` with explicit monitor; retained native scenario result 1 passed, 0 failed. |
| AC-9 | Actual bridge injection is asserted. | Read both rewritten units and SDK `Plugin.InjectWireRoute`/`DirectBridge.InjectWireRoute`: one exact call or zero short-input calls. Recorded mutation controls prove both assertions discriminate. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Config validate with redistribute. | `test/parse/bmp-route-action-refuses-redistribute.ci` | Step 1 asserts exit 1, full path, rejected value, accepted monitor and exact fix. |
| Daemon start with redistribute. | Same parse `.ci`. | Step 4 asserts startup exit 1 and the same refusal. |
| SIGHUP reload with redistribute. | `test/reload/bmp-route-action-redistribute-refused.ci` | Established UPDATE/EOR precedes rewrite/SIGHUP/SIGTERM. Refusal and exit 0 are required. |
| Monitor written or leaf absent. | Parse `.ci` steps 2/3 and `test/parse/bmp-config.ci`. | Accepted configurations assert exit 0, preventing blanket receiver refusal. |
| Editor, web, REST and gNMI generic config paths. | Closed enum-refusal parent's cited live/editor/browser tests. | A-2b resolved by that work's final multi-surface unit/live/load proof and A/B closure; no new BMP transport proof is claimed. |
| Route Monitoring from a monitored router. | Both `TestProcessRouteMonitoring` units and preserved ingest/isolation `.ci` controls. | Ready SDK bridge observes exact protocol/key/body. Real fixture routes appear in BMP results and are absent from BGP best path. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-2b | confirmed | Parent closure 13310151fb / 9b4f33044a and its retained multi-surface unit/live/load proofs. |
| A-1 | confirmed | Owner Q-1 makes monitoring the end state. No redistribution feature was deferred. |
| A-2a | confirmed | Owned validate/start/reload functional proof, with rejected and accepted values and recorded YANG-revert RED controls. |
| A-3 | confirmed | Whole tracked config census found the explicit-monitor FRR scenario and intentional negative tests. No intended-valid redistribute config survives. |
| A-4 | confirmed | `configurationPaths` copies snapshot inputs. Named site mirror run PASS, `ok github.com/ze-software/ze/internal/le/site 125.826s`. |
| A-5 | confirmed | Current field/warning absence and approved test deletion verified against source and fa74dc4c00. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Receiver accepts monitor alone. | `ze-bmp-conf.yang` `route-action` enumeration and default. | Yes, guide field table and Looking Glass Integration. |
| Monitoring remains separate from BGP best path. | `processRouteMonitoring` sends `protocolBMP`; `handleInjectWireRoute` stores by protocol. The best-path unit asserts only the BGP peer is a candidate. | Yes, guide behavior retained. |
| Feature declaration, comparison and architecture need no change. | `features/bmp-delivery.md` declares experimental/partial BMP delivery and no receiver redistribution. Searches for `route-action` in configuration/syntax/comparison/core-design pages find no BMP leaf claim. | Yes. No feature level, architecture, wire format or comparison promise changes. |
| CLI, API/RPC, SDK, inventory, metadata, metrics and test infrastructure need no change. | Source diff removes one field and warning, retains all registrations and public methods, and uses existing test directives. Existing source-anchor search reaches the BMP guide and unrelated `bmpCompositeKey` documentation only. | Yes. No new command, registration or runtime dependency. |
| RFC records and feature support are unchanged. | No owned changed test has an `RFC requirement:` tag. The final 2026-11-13 `./le rfc check` log `job-enum-final-closure-rfc-freshness-4fea72ad.log` reports 99 foreign violations and no BMP producer/unit violation. | Yes. No claim of global RFC green or complete BMP conformance. |

### Closure record notes

The original BMP state checkpoint preceded the final proofs. Its not-run
statements are historical, superseded by the committed Deliverables proof.
The old weakening-ledger instructions were obsolete: the native judge
approvals in fa74dc4c00 are authoritative, and the active design rows now
name that completed route.
No new learned-summary file is required. The schema explains why the leaf
remains, and the guide carries the operator decision.

The unchanged repo/weakened prechecks are retained from the current closure
session: `job-enum-round6-repo-check-89d513b0.log` has 31 foreign findings,
and `job-enum-round6-weakened-check-36862edf.log` has 13 foreign IKE/sysrib
findings. Neither reports an owned BMP finding. The final documentation check
retains the same seven foreign anchors and reports no owned BMP anchor error.
Unchanged runtime proof was not repeated.

### Final native checks

| Check | Command / evidence | Result |
|-------|--------------------|--------|
| Site mirror, A-4 | `./le --name session test unit pkg path ./internal/le/site/ run '^TestTheConfigurationMirrorReadsAsThePublishedMirror$'` | PASS: `ok github.com/ze-software/ze/internal/le/site 125.826s`; no failed/skipped result reported. |
| Repository configuration corpus | `./le --name session test unit pkg path ./internal/component/config/cli/ run '^TestRepositoryConfigsValidate$'` | PASS: `ok github.com/ze-software/ze/internal/component/config/cli 153.598s`; no failed/skipped result reported. |
| Documentation | `./le --name session doc check all` after the guide anchor edit. | Seven foreign anchor errors: sysrib `ApplyConfiguration`/`recomputeBest` and five report `max*Len` symbols. No owned BMP error. The index warning is handled by the following row. |
| Discovery indexes | `./le --name session doc index update` | PASS: regenerated both indexes, 382 design docs and 4718 code paths across 819 packages. No tracked index diff remained. |
| Review artifact | `./le --name session spec review record` and `spec review check`, naming every path in the artifact above. | PASS: 14 recorded paths, 13 code files, CLEAN round 1, hashes match. |

No Go source changed in closure. The two `.ci` edits change comments only,
so the completed runtime proof remains applicable. No build or unchanged
passing BMP test was repeated for confirmation. No `./le verify` ran, and no
whole-tree, global RFC or release-green result is claimed.
