# Spec: l2tp-shaper-upload-rate-is-not-enforced

| Field | Value |
|-------|-------|
| Status | done |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.**
`internal/component/l2tp/plugins/shaper/yang/ze-l2tp-shaper-conf.yang` declares
`leaf upload-rate` with the description "Default upload rate. When omitted,
defaults to default-rate." The sibling leaf is `default-rate`, described as
"Default download rate applied to a new session." A reader takes the pair to
mean Ze shapes a subscriber session in both directions, at the download rate one
way and the upload rate the other.

**What Ze does instead.** No interface enforces the upload rate.
`(*shaperPlugin).applyTC`
(`internal/component/l2tp/plugins/shaper/shaper.go`) takes one `rateBps`
argument and builds one `traffic.TrafficClass` from it, setting `Rate` and, for
HTB, `Ceil`. It has no second rate parameter, so it can install one direction
only. Both callers pass the download rate. `(*shaperPlugin).onSessionUp` in the
same file stores `state.uploadRate` on the session and then calls
`s.applyTC(payload.Interface, cfg.QdiscType, state.downloadRate)`.
`(*shaperPlugin).handleSubscriberSessionUp`, the `subscriber.ShaperHandler` that
PPPoE calls on SessionUp, takes its upload argument as `_` and calls `applyTC`
with the download rate.

The value is stored and reported, which is what makes this the operator's
problem. `(*shaperPlugin).showSessions` in the same file renders it as the
`upload-rate-bps` JSON key, so `show l2tp shaper` reads back a rate that no
queueing discipline is enforcing. The same gap swallows half of a RADIUS answer:
`onSessionUp` parses a Filter-Id of the form `rate:20mbit/5mbit` through
`parseFilterRate` (`internal/component/l2tp/plugins/shaper/filter_rate.go`),
logs both halves, stores both on the session, and applies the
download half alone. A subscriber the RADIUS server rate-limited upward is not
rate-limited upward.

**What closing it means.** The implementer chooses between building the behavior
and refusing the leaf at commit the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses `vrf`. Building it is
obviously right, because the refusal does not reach the defect: the RADIUS
Filter-Id path carries an upload rate that no leaf declares, so refusing the
leaf would leave `show l2tp shaper` reporting an unenforced rate from RADIUS
with nothing to refuse. What the design owes is the mechanism. A queueing
discipline on the `pppN` interface shapes what leaves that interface, which is
the download direction from the subscriber's point of view, so the upload
direction needs either an ingress qdisc, a policer, or shaping applied on the
other side of the tunnel. That mechanism choice is the whole difficulty, and it
belongs to the design.

`plan/to-review/spec-l2tp-12-xdp-policing.md` is at Status `ready` and builds
per-session ingress policing with XDP. It declares its own YANG and reads no
leaf under `l2tp { shaper { ... } }`, so it does not close this leaf. Whether
the two should share one mechanism is a question this spec's design must put,
because two mechanisms answering the same question is one too many
(`ai/rules/simplicity.md`).

## Research Findings (2026-09-06)

The Task section names one open question: which mechanism enforces the upload
direction, and which plugin owns it. This section answers it from the producers
and states the decision the owner must take. No code changed, and the spec stays
at `skeleton`, because every edit available takes that decision.

### Verified facts

| # | Fact | Producer read |
|---|------|---------------|
| F-1 | The shaper installs one root qdisc with one class, built from a single rate. Egress on a pppN interface carries traffic toward the subscriber, which is the download direction | `(*shaperPlugin).applyTC`, `internal/component/l2tp/plugins/shaper/shaper.go` |
| F-2 | The upload direction is ingress on pppN, and the traffic data model cannot express it. `InterfaceQoS` holds one root `Qdisc` and carries no policer | `InterfaceQoS`, `Qdisc`, `TrafficClass`, `internal/component/traffic/model.go` |
| F-3 | The netlink backend refuses the two ingress-hook qdisc kinds: "attaches at the ingress hook, not at the root, and is not configurable". So no upload enforcement is reachable through `traffic.Backend` today | `translateQdisc`, `internal/plugins/traffic/netlink/translate_linux.go` |
| F-4 | The ingress hook is already shared. The mirror path installs clsact at handle `ffff:`, and the sampling path reuses it. A pppN policer is a third owner of that hook | `internal/plugins/iface/netlink/mirror_linux.go`, `internal/plugins/flowexport/sampling/tc_linux.go` |
| F-5 | Both access types terminate on a pppN interface. The L2TP path shapes `SessionUpPayload.Interface` and the PPPoE path shapes `sess.PppInterface`. One ingress mechanism on pppN covers both | `(*shaperPlugin).onSessionUp` and `(*shaperPlugin).handleSubscriberSessionUp`, `shaper.go`; `internal/component/l2tp/pppoe/subsystem.go` |
| F-6 | `spec-l2tp-12-xdp-policing.md` polices inbound L2TP data packets on the uplink NIC, keyed by tunnel id and session id. A PPPoE session carries no L2TP header, so that design cannot enforce upload for a PPPoE subscriber | `plan/to-review/spec-l2tp-12-xdp-policing.md`, Data Flow and AC-4 |
| F-7 | That spec declares its own rate leaves under `l2tp { policing { ... } }` and takes the per-session rate from `SessionRateChangePayload.UploadRate`. It reads neither `l2tp/shaper/upload-rate` nor the RADIUS Filter-Id, and its session-up map entry uses its own YANG default | same spec, AC-6 and AC-6a |
| F-8 | `subscriber.Session.UploadRate` has no producer. No assignment exists anywhere, so the PPPoE call site always hands the shaper a zero upload rate, and `show subscriber` never prints the key | `subscriber.Session`, `internal/component/l2tp/subscriber/session.go`; `internal/component/l2tp/subscriber/cmd/subscriber.go` |
| F-9 | The `ze:help` beside the leaf already states that no interface enforces the rate. The leaf `description` and `docs/guide/l2tp.md` still read as a promise of enforcement | `internal/component/l2tp/plugins/shaper/yang/ze-l2tp-shaper-conf.yang`; `docs/guide/l2tp.md`, the Traffic shaping section |

F-8 corrects the Task section on one point. The PPPoE handler does take its
upload argument as `_`, and the argument it discards is always zero.

### The decision the owner must take

Two mechanisms answer the same question, and only one of them should exist
(`ai/rules/simplicity.md`).

| Route | Owner | What it gives | What it costs |
|-------|-------|---------------|---------------|
| A: policing on the pppN ingress hook | `l2tp-shaper` | One mechanism for L2TP and PPPoE, on the interface that already carries the download shaping, fed by the rates this plugin already parses | An ingress policer in the traffic model, netlink translation, a VPP answer, and a third owner on the shared clsact hook. It makes the line-rate bucket of spec-l2tp-12 a duplicate |
| B: XDP policing on the uplink NIC | `l2tp-policing`, spec-l2tp-12 | A design that is written and at `ready`, and drops before kernel decapsulation | A PPPoE subscriber stays unpoliced upward. `l2tp/shaper/upload-rate` and the upload half of the RADIUS Filter-Id keep no reader, so both have to move to the policing plugin or be read by it |

Under either route `show l2tp shaper` must stop reporting a rate the shaper does
not enforce. What replaces the `upload-rate-bps` key depends on the route.

### Why no code changed

Each candidate edit has a different correct form under each route: remove the
leaf, reword the leaf `description`, change or drop the show key, or install a
policer. Picking one picks the route. The route is the owner's decision
(`ai/rules/rule-precedence.md`, rung 3).

## Owner Decision (2026-09-06): route A

Thomas chose route A. The subscriber upload rate is enforced by a policer on the
`pppN` interface ingress hook, installed by the `l2tp-shaper` plugin through the
traffic backend. Route B (`plan/to-review/spec-l2tp-12-xdp-policing.md`, XDP
keyed on tunnel id and session id) is rejected: a PPPoE subscriber carries no
L2TP header, so that key cannot reach it, and F-5 already shows both access
types terminate on a `pppN`.

Two corroborating readings and two defects to design away from, taken from
osvbng (`~/Code/veesix-networks/osvbng`, read for shape, not copied):

| # | Reading | Where |
|---|---------|-------|
| C-1 | osvbng polices upstream with a per-subscriber token-bucket policer on the subscriber interface's input feature arc, named `sub_<sw_if_index>_in`. Not tc, not XDP | `(*VPP).ApplyQoS`, `pkg/southbound/vpp/qos.go` |
| C-2 | One mechanism serves both access types there for the same reason it does here: both decap nodes rewrite the RX interface to the per-session interface before the ingress arc runs, so the interface index is the natural key once the subscriber IP packet exists | `osvbng_pppoe_decap.c`, `l2tpv2_input.c` |
| C-3 | The direction asymmetry is deliberate: downstream gets queueing and AQM, upstream gets a policer, because the subscriber's upload is already constrained by the access link | `osvbng_qos_sched/INTRODUCTION.md` |
| C-4 | DEFECT to avoid: osvbng's numeric `qos.upload-rate` resolves into `ServiceGroup.UploadRate` and is read only by `LogAttrs`. Stored, shown, never programmed. That is exactly Ze's defect today | osvbng |
| C-5 | DEFECT to avoid: `ApplyQoS` opens with an install-once guard, so a CoA is accepted, merged, and never programmed. The mid-session rate-change path is designed in here from the start, not added later | osvbng |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - section 14b names the traffic component, its `Backend` interface and its data model
  → Decision: one `Backend.Apply(ctx, map[string]InterfaceQoS)` programs an interface's whole desired state, so the upload direction belongs INSIDE `InterfaceQoS` rather than beside it
  → Constraint: exact-or-reject. A backend that cannot represent a field refuses it at verify and at apply; it never programs a subset and reports success
- [ ] `docs/guide/l2tp.md` - the Traffic shaping section is the operator's promise
  → Constraint: the page read as a promise of two-direction shaping while only one direction was enforced, so it changes in the same work as the code

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2865.md` - Section 5.11 Filter-Id carries the subscriber's rate profile
  → Constraint: the Filter-Id is a free-form string, so a value that is not a rate is not an error; it means "no rate here"
- [ ] `rfc/short/rfc5176.md` - CoA-Request semantics
  → Constraint: an authorization change the NAS cannot carry out owes a CoA-NAK, so a rate the NAS accepts and does not program is a conformance defect as well as a functional one

**Key insights:**
- A qdisc shapes EGRESS by delaying a packet. The ingress hook holds no queue, so the only enforcement there is to drop, which is a policer.
- The `clsact` qdisc at handle `ffff:` is shared: mirror owns tc priority 1, flow-export sampling owns 100. A third owner adds a priority and never replaces or deletes the qdisc.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/plugins/shaper/shaper.go` - `applyTC` took ONE rate and built one root qdisc from it; all three call sites passed the download rate
- [ ] `internal/component/traffic/model.go` - `InterfaceQoS` held one root `Qdisc` and no policer, so the model could not express ingress
- [ ] `internal/plugins/traffic/netlink/translate_linux.go` - `translateQdisc` refuses `QdiscClsact` and `QdiscIngress`: "attaches at the ingress hook, not at the root, and is not configurable"
- [ ] `internal/plugins/traffic/netlink/backend_linux.go` - `applyInterface` programs root qdisc, classes, filters; `restoreOriginalLocked` puts the snapshot back
- [ ] `internal/plugins/iface/netlink/mirror_linux.go` - `clsactQdisc`, and the comment that names the hook as shared and forbids deleting it
- [ ] `internal/plugins/flowexport/sampling/tc_linux.go` - `SampleFilterPriority` 100, the second owner, and the matchall+action pattern the policer follows
- [ ] `internal/component/l2tp/plugins/authradius/coa.go` - `extractRate` open-coded a `ParseRateBps` over the whole Filter-Id and emitted `UploadRate: downloadRate`
- [ ] `internal/component/l2tp/plugins/authradius/extract_vsa.go` - `parseMikrotikRate` returned both directions; `extractVSARate` and `mikrotikRateToFilterID` dropped the second
- [ ] `internal/component/l2tp/pppoe/subsystem.go` - `onSessionUp` built a `subscriber.Session` and never set `DownloadRate` or `UploadRate`
- [ ] `internal/plugins/traffic/vpp/verify.go` - the exact-or-reject posture and the shape of a backend refusal

**Behavior to preserve:**
- `show l2tp shaper` keys, including `upload-rate-bps`
- The `l2tp/shaper` config surface: no leaf added, renamed or removed
- Every existing qdisc translation, and the refusal of clsact and ingress AT THE ROOT
- The mirror and sampling filters on the shared ingress hook

**Behavior to change:**
- The upload rate is enforced by a policer on the `pppN` ingress hook, for L2TP and PPPoE alike
- A CoA carrying an asymmetric Filter-Id is accepted rather than NAK'd, and changes both directions
- A MikroTik `Mikrotik-Rate-Limit` keeps its upload half through Access-Accept
- A PPPoE session carries the rates its RADIUS profile named

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `l2tp { shaper { upload-rate 2mbit; } }`, YANG JSON into the plugin
- RADIUS: Access-Accept `Filter-Id` or `Mikrotik-Rate-Limit`, and the same attributes in a CoA-Request

### Transformation Path
1. `parseShaperConfig` reads `upload-rate` into `shaperConfig.UploadRate`; `uploadRateOrDefault` applies the documented default
2. `traffic.ParseFilterIDRate` reads a RADIUS Filter-Id into a rate pair, for the shaper and the CoA listener alike
3. `(*shaperPlugin).applyTC` builds `traffic.InterfaceQoS{Qdisc: ..., Ingress: traffic.NewPolicer(uploadBps)}`
4. `(*backend).applyInterface` programs the root qdisc, then `applyIngressPolicer` adds the `clsact` hook and a `matchall` filter carrying a `police` action at priority 200
5. Teardown: `restoreOriginalLocked` clears priority 200 and leaves the qdisc

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Shaper plugin ↔ traffic component | `traffic.Backend.Apply` with a value-typed `InterfaceQoS` | Yes -- `TestSessionUpEnforcesUploadRate` |
| Traffic component ↔ kernel | netlink RTM_NEWTFILTER, matchall + police | Yes -- `TestNetlinkIntegration_IngressPolicerReachesTheKernel` reads it back |
| RADIUS listener ↔ shaper | `l2tpevents.SessionRateChange` carrying both rates | Yes -- `TestRateChangeUpdatesUploadRate` |
| PPPoE ↔ shaper | `subscriber.ShaperHandler(iface, download, upload)` | Yes -- `TestPPPoESessionUpCarriesRadiusRates` |

### Integration Points
- `traffic.InterfaceQoS` gains `Ingress Policer`; both backends answer for it
- `subscriber.ShaperHandler`'s third argument stops being discarded

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The shaper never speaks netlink; it fills `InterfaceQoS` and the backend translates |
| No unintended coupling (components stay isolated) | Yes | `authradius` and the shaper share `traffic.ParseFilterIDRate` rather than importing each other |
| No duplicated functionality (extends existing, does not recreate) | Yes | One `PolicerBurstBytes`, consumed by tc and VPP; the VPP `burstBytes` became a caller |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control plane, once per session |
| Registration over hardcoding | Yes | No central enumeration edited; the backends already register, and `Policer` is a model field both read |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `matchall` filter with a `police` action on the clsact ingress hook is accepted by the kernel and enforces the rate | `vendor/github.com/vishvananda/netlink/filter_linux.go`, `encodePolice`; `internal/plugins/flowexport/sampling/tc_linux.go` uses the same filter kind on the same hook | The upload rate reaches nothing and the spec is not closed | `TestNetlinkIntegration_IngressPolicerReachesTheKernel` reads the police action back from a real kernel | confirmed |
| A-2 | Adding a filter at priority 200 leaves the mirror's and sampling's filters intact | `mirror_linux.go` comment: the qdisc is shared and only filters are removed | Enabling subscriber shaping would silently disable mirroring on that interface | `TestNetlinkIntegration_IngressPolicerLeavesOtherHookOwnersAlone` | confirmed |
| A-3 | Both access types terminate on a `pppN`, so one mechanism covers both | F-5, re-read at `(*shaperPlugin).handleSubscriberSessionUp` and `pppoe/subsystem.go` | PPPoE would need a second mechanism | `TestSubscriberSessionUpEnforcesUploadRate` | confirmed |
| A-4 | The VPP backend cannot program an interface-wide ingress policer today | `internal/plugins/traffic/vpp/backend_linux.go`, `applyInterface` binds policers to the egress output arc and to the ingress classify pipeline for filtered classes only | An honest refusal would be an unnecessary regression | `TestVerifyRejectsIngressPolicer`; and `applyAll` already fails on a `pppN` with "interface not present in vpp", so the shaper never worked under VPP | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A policer whose burst is too small drops every packet | Subscriber traffic collapses at low rates | `PolicerBurstBytes` applies a 2048-byte floor, and `Policer.Validate` refuses a rate with no burst |
| R-2 | A rate above 34.359 Gbit/s truncates in the kernel's uint32 of bytes per second | Policing at a wrapped rate, which lets traffic through | `ingressPolicerFilter` refuses it; `TestIngressPolicerFilterRefusesUnrepresentableRate` pins both sides of the bound |
| R-3 | A `pppN` number is reused and the next subscriber inherits the previous policer | A subscriber limited at someone else's rate | `restoreOriginalLocked` clears the priority unconditionally, so a restart-orphaned policer is cleared too |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A subscriber's upload is dropped at the wrong rate, or an interface loses its mirror or sampling filters. Nothing outside the `pppN` interface and the shared ingress hook is touched |
| How is it reverted? | Single commit revert. No config migration: no leaf was added or renamed |
| Who else touches this path? | `internal/plugins/iface/netlink` (mirror, priority 1) and `internal/plugins/flowexport/sampling` (priority 100) on the same hook; `plan/to-review/spec-l2tp-12-xdp-policing.md` proposed the rejected alternative |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `l2tp { shaper { upload-rate 2mbit; } }` in the operator's config | → | `parseShaperConfig` then `(*shaperPlugin).applyTC` | `test/l2tp/shaper-upload-rate.ci` |
| L2TP session-up event | → | `(*shaperPlugin).onSessionUp` -> `InterfaceQoS.Ingress` | `TestSessionUpEnforcesUploadRate` |
| PPPoE session-up | → | `(*shaperPlugin).handleSubscriberSessionUp` | `TestSubscriberSessionUpEnforcesUploadRate` |
| RADIUS Access-Accept `Filter-Id: rate:20mbit/5mbit` | → | `traffic.ParseFilterIDRate` -> both rates | `TestFilterIDRateReachesBothDirections` |
| RADIUS CoA-Request carrying a rate | → | `extractRates` -> `SessionRateChangePayload.UploadRate` | `TestExtractRatesReadsAsymmetricFilterID`, `TestRateChangeUpdatesUploadRate` |
| `InterfaceQoS.Ingress` set | → | `(*backend).applyIngressPolicer` -> kernel | `TestNetlinkIntegration_IngressPolicerReachesTheKernel` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `upload-rate 2mbit` configured, an L2TP session comes up on `ppp0` | `ppp0` carries a policer at 2 Mbit/s on its ingress hook |
| AC-2 | `upload-rate` absent, `default-rate 7mbit` | The session's upload is policed at 7 Mbit/s, which is what the leaf's documented default says |
| AC-3 | RADIUS Access-Accept `Filter-Id: rate:20mbit/5mbit` | Download shaped at 20 Mbit/s and upload policed at 5 Mbit/s |
| AC-4 | A PPPoE session comes up | It gets the same two mechanisms, from the same config and the same RADIUS profile |
| AC-5 | A CoA-Request carries `Filter-Id: rate:20mbit/5mbit` for a live session | CoA-ACK, and both directions are reprogrammed. It was CoA-NAK'd before |
| AC-6 | A CoA-Request names a download rate only | The session keeps its existing upload rate rather than losing it |
| AC-7 | The session goes down | The policer is removed and the shared clsact qdisc stays, with other subsystems' filters intact |
| AC-8 | An interface asks for no upload enforcement | The shared ingress hook is not touched at all |
| AC-9 | An upload rate above 34.359 Gbit/s | Refused with a message naming the bound, never truncated |
| AC-10 | The `vpp` traffic backend is selected and an ingress policer is asked for | Refused at verify and at apply, naming what the backend cannot represent |
| AC-11 | A MikroTik `Mikrotik-Rate-Limit` of `10M/5M` in an Access-Accept | Both halves survive into the Filter-Id the shaper reads |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `upload-rate 2mbit` and starts the daemon | config -> YANG -> `parseShaperConfig` -> plugin | `test/l2tp/shaper-upload-rate.ci` |
| 2 | An L2TP subscriber connects and is limited in both directions | session-up -> `applyTC` -> `Backend.Apply` -> tc | `TestSessionUpEnforcesUploadRate` + `TestNetlinkIntegration_IngressPolicerReachesTheKernel` |
| 3 | A PPPoE subscriber connects with a RADIUS rate profile | Access-Accept -> session metadata -> `subscriber.Session` -> shaper | `TestPPPoESessionUpCarriesRadiusRates` |
| 4 | The operator raises a live subscriber's rate by CoA | CoA -> `extractRates` -> rate-change event -> `applyTC` | `TestExtractRatesReadsAsymmetricFilterID` + `TestRateChangeUpdatesUploadRate` |
| 5 | The subscriber disconnects and the interface is reused | session-down -> `RestoreOriginal` | `TestRestoreOriginalRemovesPolicerButKeepsHook` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPolicerSetDistinguishesAbsentFromConfigured` | `internal/component/traffic/model_test.go` | the absent/configured guard has a name | pass |
| `TestNewPolicerFillsBurst` | `internal/component/traffic/model_test.go` | a policer always carries a burst | pass |
| `TestPolicerBurstBytesFloor` | `internal/component/traffic/model_test.go` | AC-1 low-rate floor | pass |
| `TestPolicerValidateRejectsBurstlessRate` | `internal/component/traffic/model_test.go` | R-1 | pass |
| `TestInterfaceQoSCarriesIngressPolicer` | `internal/component/traffic/model_test.go` | the model can express the upload direction | pass |
| `TestParseFilterIDRateForms` / `...RejectsNonRates` | `internal/component/traffic/filterid_rate_test.go` | AC-3, both polarities | pass |
| `TestIngressPolicerFilterCarriesRateAndDrop` | `internal/plugins/traffic/netlink/policer_linux_test.go` | AC-1 translation | pass |
| `TestIngressPolicerFilterRefusesUnrepresentableRate` | same | AC-9, R-2 | pass |
| `TestIngressPolicerFilterRefusesBurstlessPolicer` | same | R-1 | pass |
| `TestApplyInstallsIngressPolicer` | same | AC-1, and that the qdisc is added not replaced | pass |
| `TestApplyWithoutIngressPolicerTouchesNoIngressHook` | same | AC-8 | pass |
| `TestApplyToleratesExistingClsact` | same | AC-7 coexistence | pass |
| `TestRestoreOriginalRemovesPolicerButKeepsHook` | same | AC-7 | pass |
| `TestRestoreOriginalToleratesMissingPolicer` / `...ReportsPolicerRemovalFailure` | same | the teardown error gate, both polarities | pass |
| `TestVerifyRejectsIngressPolicer` / `TestVerifyAcceptsAbsentIngressPolicer` | `internal/plugins/traffic/vpp/verify_test.go` | AC-10, both polarities | pass |
| `TestSessionUpEnforcesUploadRate` | `internal/component/l2tp/plugins/shaper/shaper_test.go` | AC-1 | pass |
| `TestSessionUpFallsBackToDefaultRateForUpload` | same | AC-2 | pass |
| `TestSessionUpEnforcesRadiusUploadHalf` | same | AC-3 | pass |
| `TestRateChangeUpdatesUploadRate` | same | AC-5 | pass |
| `TestRateChangeKeepsUploadRateWhenPayloadOmitsIt` | same | AC-6 | pass |
| `TestSubscriberSessionUpEnforcesUploadRate` / `...FallsBackToConfiguredUploadRate` | same | AC-4 | pass |
| `TestFilterIDRateReachesBothDirections` / `TestNonRateFilterIDLeavesConfiguredRates` | `internal/component/l2tp/plugins/shaper/filter_rate_test.go` | AC-3, both polarities, from the entry point | pass |
| `TestExtractRatesReadsAsymmetricFilterID` / `...RejectsNonRate` / `...KeepsMikrotikUploadHalf` | `internal/component/l2tp/plugins/authradius/coa_test.go` | AC-5, AC-11 | pass |
| `TestAccessAcceptKeepsMikrotikUploadHalf`, `TestExtractAuthMetadataMikrotikRate` | `internal/component/l2tp/plugins/authradius/extract_vsa_test.go` | AC-11 | pass |
| `TestPPPoESessionUpCarriesRadiusRates` | `internal/component/l2tp/pppoe/subsystem_test.go` | AC-4 | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `Policer.RateBps` | 1 .. 34359738360 | 34359738360 | 0 (absent, not invalid) | 34359738368 |
| `Policer.BurstBytes` | 2048 .. 4294967295 | 4294967295 | 2047 | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `shaper-upload-rate` | `test/l2tp/shaper-upload-rate.ci` | the operator configures an upload rate and the daemon starts with it reaching the plugin | pass |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | -- | -- | Nothing on the wire changed. The change is a kernel datapath policy on a local interface plus the reading of two RADIUS attributes Ze already received. The kernel is the peer, and `TestNetlinkIntegration_IngressPolicerReachesTheKernel` reads the installed state back from it | N-A |

## Files to Modify
- `internal/component/traffic/model.go` - `Policer`, `NewPolicer`, `PolicerBurstBytes`, `InterfaceQoS.Ingress`
- `internal/plugins/traffic/netlink/ops_linux.go` - `qdiscAdd` and `filterDel` on the kernel seam
- `internal/plugins/traffic/netlink/backend_linux.go` - program the policer in `applyInterface`, clear it in `restoreOriginalLocked`
- `internal/plugins/traffic/vpp/verify.go` - refuse an ingress policer at commit
- `internal/plugins/traffic/vpp/backend_linux.go` - refuse it at apply, for a caller that bypassed verify
- `internal/plugins/traffic/vpp/translate.go` - `burstBytes` becomes a caller of `traffic.PolicerBurstBytes`
- `internal/component/l2tp/plugins/shaper/shaper.go` - `applyTC` takes both rates; all three call sites pass the upload rate
- `internal/component/l2tp/plugins/shaper/config.go` - `uploadRateOrDefault`
- `internal/component/l2tp/plugins/shaper/register.go` - the configure log names both rates
- `internal/component/l2tp/plugins/shaper/yang/ze-l2tp-shaper-conf.yang` - the help stops saying no interface enforces the rate
- `internal/component/l2tp/plugins/authradius/coa.go` - `extractRates`, and both rates into the rate-change events
- `internal/component/l2tp/plugins/authradius/extract_vsa.go` - `extractVSARates`, `mikrotikRateToFilterID` keeps both halves
- `internal/component/l2tp/plugins/authradius/extract.go` - the Access-Accept caller
- `internal/component/l2tp/pppoe/subsystem.go` - the producer for `subscriber.Session`'s two rate fields
- `internal/test/fixture/tunnel_fixture_l2tp.go` - the fixture the new `.ci` drives
- `docs/guide/l2tp.md` - the Traffic shaping section
- `docs/architecture/core-design.md` - section 14b, the traffic data model
- `docs/architecture/l2tp/cos-vendor-radius.md` - the "MikroTik upload rate is discarded" consequence is no longer true
- `docs/architecture/l2tp/bng-1-radius-attributes.md` - `traffic.ParseFilterIDRate` is the one reader of a Filter-Id rate
- `docs/architecture/traffic/fw-7-traffic-vpp.md` - the ingress policer joins the exact-or-reject list
- `docs/architecture/traffic/tc-original-qdisc-restore.md` - `qdiscAdd` and `filterDel` on the `tcOps` seam, and what restore clears
- `docs/architecture/l2tp/bng-5-pppoe.md` - where a PPPoE session reads its RADIUS rate profile

## Files to Create
- `internal/component/traffic/filterid_rate.go` - `ParseFilterIDRate`, the one reader of a RADIUS Filter-Id rate
- `internal/plugins/traffic/netlink/policer_linux.go` - the tc ingress policer
- `test/l2tp/shaper-upload-rate.ci` - the operator's half of the chain

## Files Removed
- `internal/component/l2tp/plugins/shaper/filter_rate.go` - superseded by `traffic.ParseFilterIDRate`, which both consumers can reach

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No leaf added, renamed or removed. `upload-rate` already existed; this makes it do what it says |
| YANG validation constraints | No | `zt:rate` already constrains the leaf |
| YANG custom validators | No | `verifyShaperConfig` already parses the leaf at commit |
| CLI commands/flags | No | `show l2tp shaper` keys are unchanged |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | No new leaf |
| Functional test for new RPC/API | Yes | `test/l2tp/shaper-upload-rate.ci` |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | No | The clsact qdisc and the police action need no module beyond `sch_ingress`/`act_police`, which the existing mirror and sampling paths already depend on and which no doctor check covers today. Adding one would cover three subsystems, not this one, so it belongs to whoever covers the hook |
| Prometheus counters/metrics | No | None added. The kernel counts policed drops in `tc -s filter`, which is where an operator reads them |
| BGP family surface | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | `docs/features.md` already lists L2TP traffic shaping; this makes an advertised leaf work rather than adding a feature |
| 2 | Config syntax changed? | No | Same leaves, same syntax |
| 3 | CLI command added/changed? | No | -- |
| 4 | API/RPC added/changed? | No | -- |
| 5 | Plugin added/changed? | No | No plugin added or removed |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md`, Traffic shaping: both directions, the mechanism for each, and the accepted Filter-Id forms |
| 7 | Wire format changed? | No | -- |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 2865 Section 5.11 and RFC 5176 were already claimed and remain so; the CoA path now carries out the change it acknowledges, which is what the existing claim already said |
| 10 | Test infrastructure changed? | No | One fixture registered in an existing registry |
| 11 | Affects daemon comparison? | No | -- |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` section 14b: the traffic model now carries both directions |
| 13 | Route metadata keys added/changed? | No | -- |
| 14 | Prometheus counters added/changed? | No | -- |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | -- |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED from `./le spec citation anchors`. Six docs are DECLARED by changed files. Five carry an edit and are named under Files to Modify: `cos-vendor-radius.md` (its "the MikroTik upload rate is discarded" consequence became false), `bng-1-radius-attributes.md` (names the one Filter-Id rate reader), `fw-7-traffic-vpp.md` (the ingress policer joins the exact-or-reject list), `tc-original-qdisc-restore.md` (two new `tcOps` calls, and what restore clears). Five carry an edit, the fifth being `docs/architecture/l2tp/bng-5-pppoe.md` (where a PPPoE session reads its RADIUS rate profile). One is unaffected: `docs/research/l2tpv2-ze-integration.md` is the original research note describing the shaper's registration and events, neither of which changed. Of the twelve advisory MENTIONS, `docs/guide/traffic-control.md` and `docs/architecture/traffic/followup-vpp-traffic.md` were read and describe the operator's qdisc config surface and the VPP rejection RATIONALE, neither of which this change alters beyond the `fw-7` row already added; `docs/guide/pppoe.md`, `docs/guide/configuration.md`, `docs/comparison.md`, `docs/features/rfc-status.md`, `docs/functional-tests.md`, `docs/architecture/traffic/cos-dynamic.md`, `docs/architecture/traffic/cp-survival-3-egress-cs6-sched.md`, `docs/architecture/traffic/fw-7b-backend-hardening.md`, `docs/architecture/l2tp/subscriber-session-model.md` and `docs/architecture/l2tp/cos-vendor-radius.md` (already named) make no claim this change falsifies |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `docs/guide/l2tp.md` example was checked against the YANG and is unchanged, because no leaf changed |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- give the model a way to express the upload direction
   - Tests: `TestInterfaceQoSCarriesIngressPolicer`, `TestPolicerSetDistinguishesAbsentFromConfigured`, `TestNewPolicerFillsBurst`, `TestPolicerBurstBytesFloor`, `TestPolicerValidateRejectsBurstlessRate`
   - Files: `internal/component/traffic/model.go`
   - Verify: RED with `undefined: Policer`; then green
2. **Phase: the tc ingress policer** -- translate and program it
   - Tests: the `policer_linux_test.go` set, then `TestNetlinkIntegration_IngressPolicerReachesTheKernel` and `...LeavesOtherHookOwnersAlone`
   - Files: `policer_linux.go`, `ops_linux.go`, `backend_linux.go`
   - Verify: RED on the whole set; then green, including against a real kernel
3. **Phase: the VPP answer** -- refuse what it cannot program
   - Tests: `TestVerifyRejectsIngressPolicer`, `TestVerifyAcceptsAbsentIngressPolicer`
   - Files: `vpp/verify.go`, `vpp/backend_linux.go`, `vpp/translate.go`
   - Verify: RED on the refusal; then green
4. **Phase: the shaper** -- both directions at every call site
   - Tests: the seven upload tests in `shaper_test.go`
   - Files: `shaper.go`, `config.go`
   - Verify: seven RED for the same reason; then green
5. **Phase: the rate-change path** -- one Filter-Id reader, both rates through CoA
   - Tests: the `coa_test.go` and `extract_vsa_test.go` sets, `filter_rate_test.go`
   - Files: `traffic/filterid_rate.go`, `authradius/coa.go`, `authradius/extract_vsa.go`, `authradius/extract.go`, `pppoe/subsystem.go`
   - Verify: RED; then green
6. **Phase: the operator's half** -- docs, YANG help, functional test
   - Tests: `test/l2tp/shaper-upload-rate.ci`
   - Files: the YANG, `docs/guide/l2tp.md`, `docs/architecture/core-design.md`, the fixture registry
   - Verify: functional RED under a probe that ignores the leaf; then green

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has product code: AC-1..AC-3 `applyTC`, AC-4 `handleSubscriberSessionUp` + `pppoe/subsystem.go`, AC-5/AC-6 `onSessionRateChange` + `extractRates`, AC-7/AC-8 `applyIngressPolicer`/`removeIngressPolicer`, AC-9 `ingressPolicerFilter`, AC-10 `vpp/verify.go`, AC-11 `mikrotikRateToFilterID` |
| Feature completeness | Both access types reach the policer, and the RADIUS profile reaches both of them |
| Correctness | The kernel carries bytes per second, not bits: the translation divides by 8, and the integration test asserts the byte figure |
| Correctness | Exceeding traffic is dropped, because the ingress hook has no queue to delay into |
| Naming | `Ingress` names the direction, not the mechanism, so a future backend can implement it its own way |
| Data flow | The shaper speaks no netlink; the tc priority allocation lives with the backend that owns the hook |
| Rule: `ai/rules/principles.md` | The zero `Policer` is a guard with a name (`Set`), a comment and a test |
| Rule: exact-or-reject | VPP refuses rather than programming the egress half and reporting success |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The upload rate reaches a real kernel | `./le integration traffic` |
| The operator's config reaches the plugin | `./le functional l2tp` |
| No mutation marker left behind | `grep -rn MUTATION-APPLIED internal/ test/ docs/` returns nothing |
| The leaf's help no longer says nothing enforces it | `grep -n "no interface enforces" internal/component/l2tp/plugins/shaper/yang/ze-l2tp-shaper-conf.yang` returns nothing |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A RADIUS server is a remote party. `ParseFilterIDRate` accepts only a number and a known suffix, `traffic.ParseRateBps` bounds the value, and `ingressPolicerFilter` refuses a rate the kernel cannot carry rather than truncating it. A truncated rate is the dangerous direction: it lets traffic through |
| Resource exhaustion | One filter per subscriber interface, removed on teardown. `restoreOriginalLocked` clears the priority unconditionally, so a policer orphaned by a restart is cleared when the interface is reused |
| Failing open | A policer the backend cannot program is an error, never a silent skip. VPP refuses; tc refuses an unrepresentable rate |

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

- **The direction asymmetry is the design, not a compromise.** Downstream gets a queue because the BNG is the bottleneck there. Upstream gets a policer because the subscriber's access link already constrains them and the router has nothing to queue into on ingress. osvbng reached the same split independently (`osvbng_qos_sched/INTRODUCTION.md`).
- **The interface index is the natural key once the subscriber's IP packet exists.** By the time the packet reaches the ingress feature arc, the tunnel and session ids are gone. That is why route B could not serve PPPoE, and it is the same reason osvbng names its policer `sub_<sw_if_index>_in`.
- **An install-once guard is how this defect reappears.** osvbng accepts a CoA, merges it into the session, and never programs it, because `ApplyQoS` returns early when the policer exists. Every path here reprograms, and `TestRateChangeUpdatesUploadRate` pins it.
- **A shared kernel hook needs a priority allocation, and Ze now has three owners.** Mirror 1, sampling 100, policer 200. Each is declared in its own package with a comment naming the others. A fourth owner should turn that into one declaration rather than a fourth comment.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Policer on the `pppN` ingress hook, owned by the shaper | XDP on the uplink NIC keyed by tunnel and session id (`plan/to-review/spec-l2tp-12-xdp-policing.md`) | A PPPoE subscriber carries no L2TP header, so that key cannot reach half the subscribers. Owner decision, 2026-09-06 |
| `Ingress Policer` as a value field on `InterfaceQoS` | A pointer field; a separate `Backend` method | A value keeps the cross-boundary payload pointer-free, and one `Apply` still programs an interface's whole desired state |
| `Policer.Set()` names the absent/configured distinction | Reading `RateBps > 0` at each call site | A zero that another branch relies on is a guard, and a guard gets a name, a comment and a test |
| tc priority 200, after the two existing owners | 50, between mirror and sampling | The kernel stops at the first filter that returns a verdict, so running last is the only placement that changes neither existing owner's behavior |
| Add the clsact qdisc, never replace it; remove only our priority | `qdiscReplace`, or deleting the qdisc on teardown | Both would drop every mirror and sampling filter on the interface |
| `traffic.ParseFilterIDRate` shared by the shaper and the CoA listener | Leaving the CoA listener's open-coded `ParseRateBps` | The CoA path NAK'd the exact value the Access-Accept path accepts, so an asymmetric rate could be set at login and never changed. That blocks AC-5 |
| The VPP backend refuses an ingress policer | Silently programming the egress half | Exact-or-reject, and a silently unenforced upload rate is the defect this spec closes |
| `PolicerBurstBytes` declared once, consumed by tc and VPP | A second copy in the tc backend | A burst is one fact about a rate. Two copies drift |

## Known Limitations

- **The VPP backend does not police ingress.** It refuses instead. The L2TP shaper already could not run under VPP, because a Linux `pppN` is not a VPP interface and `applyAll` fails on it. Implementing a VPP subscriber input-arc policer is a separate piece of work and is not needed until subscriber termination moves into VPP.
- **The tc filter priority allocation is declared in three packages.** Mirror, sampling and the policer each declare their own constant with a comment naming the other two. One shared declaration would be better and touches three plugins, so it is not folded into this change.
- **`show l2tp shaper` reports the rates the shaper holds, not the rates the kernel holds.** Reading the installed policer back for the show command is a separate improvement; the integration test reads it from the kernel instead.

## RFC Documentation (Scope: protocol)

RFC 2865 Section 5.11 (Filter-Id) and RFC 5176 Section 2.3 (CoA) are cited above
the code that enforces them, in `shaper.go`, `coa.go` and
`internal/component/traffic/filterid_rate.go`. No new MUST is implemented: the
CoA path now carries out the authorization change it already acknowledged.

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

---

## Implementation Summary

### What Was Implemented
- `traffic.Policer`, `NewPolicer`, `PolicerBurstBytes`, `Policer.Set`, `Policer.Validate`, and `InterfaceQoS.Ingress` (`internal/component/traffic/model.go`). The data model can now express the upload direction, which it could not before.
- `traffic.ParseFilterIDRate` (`internal/component/traffic/filterid_rate.go`): the one reader of a RADIUS Filter-Id rate, shared by the shaper and the CoA listener.
- The tc ingress policer (`internal/plugins/traffic/netlink/policer_linux.go`): `ingressClsactQdisc`, `ingressPolicerFilter`, `(*backend).applyIngressPolicer`, `(*backend).removeIngressPolicer`, `isPolicerAbsent`. It adds the shared `clsact` qdisc rather than replacing it, installs a `matchall` filter carrying a `police` action at priority 200, and clears only that priority on teardown.
- `qdiscAdd` and `filterDel` on the `tcOps` seam (`ops_linux.go`), called from `applyInterface` and `restoreOriginalLocked` (`backend_linux.go`).
- The VPP refusal: `errIngressPolicerNotSupportedByBackend` at verify (`vpp/verify.go`) and at apply (`vpp/backend_linux.go`), and `vpp/translate.go`'s `burstBytes` became a caller of `traffic.PolicerBurstBytes` rather than a second copy of the derivation.
- `(*shaperPlugin).applyTC` takes both rates and programs both directions; `onSessionUp`, `onSessionRateChange` and `handleSubscriberSessionUp` each pass an upload rate, and `shaperConfig.uploadRateOrDefault` applies the leaf's documented default.
- `extractRates` and `extractVSARates` (`authradius/coa.go`, `extract_vsa.go`) return both directions, `mikrotikRateToFilterID` writes both halves, and `pppoe/subsystem.go` became the first producer of `subscriber.Session.DownloadRate` and `.UploadRate`.

### Bugs Found/Fixed
- **The CoA listener refused the value the Access-Accept path accepts.** `extractRate` open-coded `traffic.ParseRateBps` over the whole Filter-Id, so `rate:20mbit/5mbit` parsed at login and answered CoA-NAK mid-session. Fixed by making `traffic.ParseFilterIDRate` the one reader; covered by `TestExtractRatesReadsAsymmetricFilterID`.
- **`subscriber.Session.UploadRate` had no producer.** Every PPPoE session handed the shaper a zero upload rate. Fixed in `pppoe/subsystem.go`; covered by `TestPPPoESessionUpCarriesRadiusRates`.
- **`ParseRateBps` multiplied before it bounded** (`internal/component/traffic/config.go`), found by this closure's security review. `ParseRateBps("18446744074gbit")` returned `290448384, nil`: a plausible 290 Mbit/s that every downstream bound accepts, reachable from any RADIUS server that writes a Filter-Id or a MikroTik Rate-Limit. Fixed here by comparing the stated number against `math.MaxUint64/rs.mult` before the multiply; covered by `TestParseRateBpsRefusesOverflowingRate`, whose RED was observed with the guard disabled.

### Documentation Updates
- `docs/guide/l2tp.md`, Traffic shaping: both directions, the mechanism for each, and the accepted Filter-Id forms (`2be6e0b19`).
- `docs/architecture/core-design.md`, the traffic-model section: `InterfaceQoS` carries both directions, and the tc and VPP answers. Landed in `e6ef65b0d` because another session held an unrelated hunk in that file when `2be6e0b19` was prepared. Anchor: `<!-- source: internal/plugins/traffic/netlink/policer_linux.go -- ingress policer translation, priority allocation, teardown -->`.
- `docs/architecture/l2tp/cos-vendor-radius.md`, `bng-1-radius-attributes.md`, `bng-5-pppoe.md`, `docs/architecture/traffic/fw-7-traffic-vpp.md`, `tc-original-qdisc-restore.md` (`2be6e0b19`), each named by a `<!-- source: -->` anchor on a changed file.
- `./le doc check verify` fails on 3939 findings, none of them from this change: the BGP command surface, the `docs/DESIGN.md` Shipped Plugins table, and the published `../gh-pages/reference/command-equivalents/` tree. Grepped for `l2tp|shaper|policer|component/traffic`: 86 hits, all under `../gh-pages/`, all about `show l2tp` commands this change did not touch.

### Deviations from Plan
- The `docs/architecture/core-design.md` edit shipped in a second commit (`e6ef65b0d`) rather than in `2be6e0b19`, because a concurrent session held an unrelated hunk in that file. The page is at HEAD and carries the anchor.
- Two comment citations of this spec's PATH (`internal/component/l2tp/plugins/shaper/filter_rate_test.go`, `internal/plugins/traffic/vpp/verify.go`) were restated as the bare stem at closure, because commit B removes the file the path names.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The Security Review Checklist asserted "`traffic.ParseRateBps` bounds the value", and the design rested on it: `ingressPolicerFilter` refuses a rate above the kernel's maximum, which only helps if the number reaching it is the number the RADIUS server stated | `ParseRateBps` multiplied the parsed count by the suffix with no check, so a large count wrapped modulo 2^64 into a small plausible rate that every later bound accepts | this closure's security review, by reading the producer rather than the checklist row | fixed at the source, both polarities tested, row in `plan/journal/bound-wraps-before-it-refuses.md` |
| approach | `./le integration traffic` was taken as evidence that the policer reaches a real kernel | The action carries no `-v`, so it reports green over two tests that SKIP for want of CAP_NET_ADMIN. Both PASS under `sudo -n` | ran the two tests by name with `-v` before quoting the green | evidence re-taken under privilege, row in `plan/journal/gate-verdict-depends-on-the-machine.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| An interface enforces the upload rate | Done | `(*backend).applyIngressPolicer`, `internal/plugins/traffic/netlink/policer_linux.go` | matchall + police on the clsact ingress hook |
| Both callers of `applyTC` pass the upload rate | Done | `onSessionUp`, `handleSubscriberSessionUp`, `onSessionRateChange`, `internal/component/l2tp/plugins/shaper/shaper.go` | all three, not two: the rate-change path was the third |
| The RADIUS Filter-Id upload half reaches an interface | Done | `traffic.ParseFilterIDRate`, `internal/component/traffic/filterid_rate.go` | one reader for the shaper and the CoA listener |
| `show l2tp shaper` stops reporting an unenforced rate | Done | `(*shaperPlugin).showSessions`, `shaper.go` | the `upload-rate-bps` key is unchanged and is now enforced, which is what the owner decision required |
| The mechanism choice is settled and not duplicated | Done | Owner Decision, route A | `plan/to-review/spec-l2tp-12-xdp-policing.md` is the rejected alternative and is untouched |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSessionUpEnforcesUploadRate`, `TestIngressPolicerFilterCarriesRateAndDrop`, `TestApplyInstallsIngressPolicer` | |
| AC-2 | Done | `TestSessionUpFallsBackToDefaultRateForUpload` | `shaperConfig.uploadRateOrDefault` |
| AC-3 | Done | `TestSessionUpEnforcesRadiusUploadHalf`, `TestFilterIDRateReachesBothDirections` | |
| AC-4 | Done | `TestSubscriberSessionUpEnforcesUploadRate`, `TestSubscriberSessionUpFallsBackToConfiguredUploadRate`, `TestPPPoESessionUpCarriesRadiusRates` | |
| AC-5 | Done | `TestExtractRatesReadsAsymmetricFilterID`, `TestRateChangeUpdatesUploadRate` | |
| AC-6 | Done | `TestRateChangeKeepsUploadRateWhenPayloadOmitsIt` | |
| AC-7 | Done | `TestRestoreOriginalRemovesPolicerButKeepsHook`, `TestNetlinkIntegration_IngressPolicerLeavesOtherHookOwnersAlone` | |
| AC-8 | Done | `TestApplyWithoutIngressPolicerTouchesNoIngressHook` | |
| AC-9 | Done | `TestIngressPolicerFilterRefusesUnrepresentableRate`, and `TestParseRateBpsRefusesOverflowingRate` for the parser that feeds it | the second test was added at closure; without it the bound was reachable with a wrapped number |
| AC-10 | Done | `TestVerifyRejectsIngressPolicer`, `TestVerifyAcceptsAbsentIngressPolicer` | |
| AC-11 | Done | `TestExtractRatesKeepsMikrotikUploadHalf`, `TestAccessAcceptKeepsMikrotikUploadHalf`, `TestExtractAuthMetadataMikrotikRate` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every row of the Unit Tests table | Done | as named in that table | all present and PASS in the closure run of the five packages: 440 PASS, 0 FAIL, 0 SKIP |
| `TestNetlinkIntegration_IngressPolicerReachesTheKernel` | Done | `internal/plugins/traffic/netlink/integration_linux_test.go` | PASS under `sudo -n`; SKIPs unprivileged, which is what the Mistake Log row records |
| `TestNetlinkIntegration_IngressPolicerLeavesOtherHookOwnersAlone` | Done | same | PASS under `sudo -n` |
| `shaper-upload-rate` | Done | `test/l2tp/shaper-upload-rate.ci` | PASS, case 26 of `./le functional l2tp` |
| `TestParseRateBpsRefusesOverflowingRate` | Changed | `internal/component/traffic/config_test.go` | not in the plan; added by this closure's security review |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify | Done | `docs/architecture/core-design.md` landed in `e6ef65b0d`; the rest in `2be6e0b19` |
| Every file in Files to Create | Done | `filterid_rate.go`, `policer_linux.go`, `test/l2tp/shaper-upload-rate.ci` all exist |
| `internal/component/l2tp/plugins/shaper/filter_rate.go` | Done | removed, as planned |
| `internal/component/traffic/config.go`, `config_test.go` | Changed | not in the plan; the `ParseRateBps` overflow guard the security review found |

### Audit Summary
- **Total items:** 11 AC, 5 Task requirements, 32 planned tests, 24 planned files
- **Done:** all of them
- **Partial:** none
- **Skipped:** none
- **Changed:** 2, both recorded in Deviations and in the Mistake Log: the `core-design.md` page landed in a second commit, and the `ParseRateBps` overflow guard was added beyond the plan

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The configured `upload-rate` is enforced by an interface, not merely stored and shown | functional + kernel integration | `test/l2tp/shaper-upload-rate.ci` PASS (case 26, `./le functional l2tp`, 25/25 pass) carries the operator's config to the plugin; `TestNetlinkIntegration_IngressPolicerReachesTheKernel` PASS under `sudo -n` reads the `police` action back off a real ingress hook |
| A RADIUS-authorized upload rate reaches the same interface | functional + unit over the entry point | `TestFilterIDRateReachesBothDirections` drives the shaper's own session-up path with `rate:20mbit/5mbit` and asserts both halves reach an interface; `TestPPPoESessionUpCarriesRadiusRates` does it for the PPPoE access type |
| A mid-session CoA changes both directions rather than being NAK'd | unit over the CoA entry point | `TestExtractRatesReadsAsymmetricFilterID` (the value that was NAK'd) plus `TestRateChangeUpdatesUploadRate` and `TestRateChangeKeepsUploadRateWhenPayloadOmitsIt` |
| The shared kernel hook keeps its other owners | kernel integration | `TestNetlinkIntegration_IngressPolicerLeavesOtherHookOwnersAlone` PASS under `sudo -n`: a foreign filter at another priority survives install and teardown |
| One mechanism serves both access types, not two | design + test | `applyTC` is the single programmer for L2TP and PPPoE (`TestSessionUpEnforcesUploadRate` and `TestSubscriberSessionUpEnforcesUploadRate` reach it from the two different entry points). Route B stays unimplemented and is named as rejected |
| A backend that cannot enforce it refuses rather than half-programs | unit, both polarities | `TestVerifyRejectsIngressPolicer` and `TestVerifyAcceptsAbsentIngressPolicer` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | Every AC has product code and a passing test. The three Known Limitations are decisions the owner took, not in-scope items left undone: the VPP ingress policer is refused by design (AC-10), the tc priority allocation stays declared per package because one shared declaration touches three plugins that this spec does not own, and `show l2tp shaper` reports the shaper's own state by design, with the kernel read covered by the integration test | -- |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/l2tp-shaper-upload-rate-is-not-enforced-d64e7b3f-bfdc-4614-8db4-f12043eb77cc.md`, 6 files, verdict=clean |
| `./le spec session review check` | `review_gate: OK (5 code files, clean, hashes match ...)` |
| Rounds | 2. Round 1 read the complete diff of `2be6e0b19` plus the closure edits and found one ISSUE and two NOTEs; round 2 re-read the fixes and re-ran the touched packages and the lint, and found nothing |
| Reviewer lenses used | untrusted-input and bounds (a RADIUS server writes the rate string); shared-resource ownership (three subsystems on one clsact hook); zero-value-as-guard (`Policer.Set`); wiring from each entry point to the producer; `docs/contributing/ze-go-style.md` step-18 style pass over every changed Go file; closure-deletes-a-cited-document |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The bound at `ingressPolicerFilter` is defeated before it runs. `ParseRateBps` multiplies the parsed count by the suffix with no check, so `"18446744074gbit"` returned `290448384, nil`: a plausible 290 Mbit/s inside every downstream bound. The string comes from a RADIUS server, and the spec's own Security Review Checklist claimed this function bounds the value | `ParseRateBps`, `internal/component/traffic/config.go` | compare the stated count against `math.MaxUint64/rs.mult` before the multiply; `TestParseRateBpsRefusesOverflowingRate` pins the wrapping input and the largest that does not wrap, RED observed with the guard disabled |
| 2 | NOTE | `rateBps := downloadBps` is an alias with no purpose, left by the rename. Two names for one fact (`docs/contributing/ze-go-style.md`, "State that goes stale") | `(*shaperPlugin).applyTC`, `internal/component/l2tp/plugins/shaper/shaper.go` | the alias is deleted and the three uses read `downloadBps` |
| 3 | NOTE | Two comments cite this spec by PATH. Commit B removes the file, so `./le doc check links` and `DesignReferences` would resolve a path that is gone | `filter_rate_test.go`, `internal/plugins/traffic/vpp/verify.go` | restated as the bare stem `spec-l2tp-shaper-upload-rate-is-not-enforced`, which keeps the name and drops a resolution promise the tree cannot keep |

Findings NOT fixed, because they are not this change: `./le repository check` reports `ExtractRemovePrivateASOps` (`internal/component/bgp/reactor/filter_delta.go`) has no cross-package non-test caller, in another session's uncommitted hunk. `./le verify lint run` reports roughly 70 findings across `bgp`, `cli`, `ike`, `internal/test/fixture` and `internal/le`, none in a file this spec touches after finding 1's `nestingReduce` was fixed. `./le doc check links` reports 27 broken references and `./le spec citation` 8 dangling ones, none naming this spec or a file it changed.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/traffic/filterid_rate.go` | Yes | `ls -l`: `-rw-rw-r-- 1 thomas thomas 1730 Sep 6 21:37` |
| `internal/plugins/traffic/netlink/policer_linux.go` | Yes | `ls -l`: `-rw-rw-r-- 1 thomas thomas 7617 Sep 6 21:48` |
| `test/l2tp/shaper-upload-rate.ci` | Yes | `ls -l`: `-rw-rw-r-- 1 thomas thomas 1649 Sep 6 21:51` |
| `internal/component/l2tp/plugins/shaper/filter_rate.go` | No, as planned | `ls`: `No such file or directory`; superseded by `traffic.ParseFilterIDRate` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | a policer at the configured rate on the ingress hook | `--- PASS: TestSessionUpEnforcesUploadRate`, `--- PASS: TestApplyInstallsIngressPolicer (0.16s)` |
| AC-2 | absent leaf falls back to the download rate | `--- PASS: TestSessionUpFallsBackToDefaultRateForUpload` |
| AC-3 | `rate:20mbit/5mbit` reaches both directions | `--- PASS: TestSessionUpEnforcesRadiusUploadHalf`, `--- PASS: TestFilterIDRateReachesBothDirections` |
| AC-4 | a PPPoE session gets both mechanisms from the same profile | `--- PASS: TestSubscriberSessionUpEnforcesUploadRate`, `--- PASS: TestPPPoESessionUpCarriesRadiusRates` |
| AC-5 | CoA-ACK, both directions reprogrammed | `--- PASS: TestExtractRatesReadsAsymmetricFilterID`, `--- PASS: TestRateChangeUpdatesUploadRate` |
| AC-6 | a download-only CoA keeps the existing upload rate | `--- PASS: TestRateChangeKeepsUploadRateWhenPayloadOmitsIt` |
| AC-7 | teardown clears the policer and keeps the hook | `--- PASS: TestRestoreOriginalRemovesPolicerButKeepsHook (0.26s)`, and `--- PASS: TestNetlinkIntegration_IngressPolicerLeavesOtherHookOwnersAlone (0.08s)` under `sudo -n` |
| AC-8 | no upload enforcement leaves the hook untouched | `--- PASS: TestApplyWithoutIngressPolicerTouchesNoIngressHook (0.15s)` |
| AC-9 | a rate past the kernel's maximum is refused, never truncated | `--- PASS: TestIngressPolicerFilterRefusesUnrepresentableRate`, `--- PASS: TestParseRateBpsRefusesOverflowingRate` |
| AC-10 | vpp refuses at verify and at apply | `--- PASS: TestVerifyRejectsIngressPolicer`, `--- PASS: TestVerifyAcceptsAbsentIngressPolicer` |
| AC-11 | a MikroTik `10M/5M` keeps both halves | `--- PASS: TestExtractRatesKeepsMikrotikUploadHalf`, `--- PASS: TestAccessAcceptKeepsMikrotikUploadHalf` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `l2tp { shaper { upload-rate 2mbit; } }` in the operator's config | `test/l2tp/shaper-upload-rate.ci` | Yes. Read the file: it pipes the config into `ze -`, drives `ze-test fixture l2tp/shaper-upload-rate`, and asserts `upload-rate=2000000` on the daemon's own `l2tp-shaper: configured` line, which `runPlugin` writes from `pending.uploadRateOrDefault()`. PASS as case 26 of `./le functional l2tp` |
| L2TP session-up | -- | Yes. `TestSessionUpEnforcesUploadRate` drives `onSessionUp` and reads `InterfaceQoS.Ingress` off the backend it installs |
| PPPoE session-up | -- | Yes. `TestSubscriberSessionUpEnforcesUploadRate` drives the registered `subscriber.ShaperHandler` |
| RADIUS Access-Accept `Filter-Id` | -- | Yes. `TestFilterIDRateReachesBothDirections` stores the metadata and enters through session-up, not through the parser |
| RADIUS CoA-Request | -- | Yes. `TestExtractRatesReadsAsymmetricFilterID` reads the attributes off a built packet; `TestRateChangeUpdatesUploadRate` carries the payload to `applyTC` |
| `InterfaceQoS.Ingress` set | -- | Yes. `TestNetlinkIntegration_IngressPolicerReachesTheKernel` reads the `police` action back from a namespace's own tc state |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `--- PASS: TestNetlinkIntegration_IngressPolicerReachesTheKernel (0.23s)` under `sudo -n`. The kernel accepts the matchall + police filter and reports it back |
| A-2 | confirmed | `--- PASS: TestNetlinkIntegration_IngressPolicerLeavesOtherHookOwnersAlone (0.08s)` under `sudo -n` |
| A-3 | confirmed | Both access types reach `applyTC`: `--- PASS: TestSessionUpEnforcesUploadRate` and `--- PASS: TestSubscriberSessionUpEnforcesUploadRate`, from two different entry points |
| A-4 | confirmed | `--- PASS: TestVerifyRejectsIngressPolicer` and `--- PASS: TestVerifyAcceptsAbsentIngressPolicer`, and `applyInterface` carries the same refusal for a caller that bypasses verify |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/l2tp.md` Traffic shaping describes both directions | read against `applyTC` and `uploadRateOrDefault`; the config example was checked against the YANG, which no leaf changed | Yes |
| `docs/architecture/core-design.md` traffic-model section | the paragraph at the `<!-- source: internal/plugins/traffic/netlink/policer_linux.go -->` anchor names priority 200 behind mirror 1 and sampling 100, which is what `policerFilterPriority` and the two sibling constants say | Yes, at HEAD in `e6ef65b0d` |
| `docs/architecture/l2tp/cos-vendor-radius.md`: "the MikroTik upload rate is discarded" | falsified by `mikrotikRateToFilterID`, which now writes both halves; the page was edited in `2be6e0b19` | Yes |
| Categories answered No | `grep -rn ParseRateBps docs/ ai/` returns one hit, in `docs/architecture/l2tp/bng-1-radius-attributes.md`, which describes the open-coded call in the past tense and stays correct after the closure's parser fix. No CLI command, RPC, YANG leaf, plugin, wire format or metric changed, so rows 1-5, 7-8, 10-11, 13-15 stay No | Yes |
| RFC status row | No RFC-level behavior newly implemented or newly proven. RFC 2865 Section 5.11 and RFC 5176 Section 2.3 were already claimed; the CoA path now carries out the change it already acknowledged | Yes, no `rfc/short/` Meta row edited |
| `./le doc check verify` | fails on 3939 findings. Grepped for `l2tp|shaper|policer|component/traffic`: 86 hits, every one under `../gh-pages/reference/command-equivalents/` about `show l2tp` and `clear l2tp` commands this change does not touch. None from this spec | Yes, none owed here |

## Core Insight

A config leaf that is parsed, validated, stored and printed passes every signal
an operator or a reviewer has. Only the producer at the far end says whether
anything enforces it, and there is no signal at all when the answer is nothing.
That is why `plan/journal/unwired-feature.md` had already named this exact leaf
in a sweep two days before the spec was written, and why the sweep found it by
asking who READS a leaf rather than by asking whether it works.

The mechanism half has its own lesson. The two directions of one subscriber
interface are not one problem solved twice. Egress owns a queue and can shape by
delaying a packet; ingress has no queue, so the only enforcement there is to
drop. A model that carries one `Qdisc` per interface cannot express the second
one, and the missing enforcement was downstream of that missing field.
