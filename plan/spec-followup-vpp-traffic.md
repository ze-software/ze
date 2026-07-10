# Spec: followup-vpp-traffic

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1-2 done (protocol); 5 done (mark rejected); 3 done (dscp police-by-dscp); 6 done (multi-class steering); 4 resolved (prio rejection-retained) |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/traffic/vpp/` (verify.go, translate.go, backend_linux.go, ops.go)
4. `internal/plugins/firewall/vpp/classify_linux.go` - the classify prior art to reuse
5. `internal/plugins/iface/vpp/ifacevpp.go` (enableVLANQoS :334) - the 3-step QoS prior art
6. `plan/learned/627-fw-7-traffic-vpp.md`, `ai/rationale/exact-or-reject.md` - design history
7. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Wire the VPP classify + QoS pipeline for traffic-control. The govpp classify/qos binapi packages are vendored, so these are design-unblocked but code-unwired; the verifier rejects each feature at config-verify time (exact-or-reject) rather than shipping a silent no-op.

This was a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Designed 2026-07-09; all evidence re-verified at that date.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **filter protocol end-to-end (L207,L208)** - classify table create + `ClassifySetInterfaceIPTable` attach + session mgmt + IPv6 next-header (L207). First impl attached to no interface and matched wrong offset.
- **filter dscp 3-step QoS (L209)** - needs `QosRecordEnableDisable` (ingress) + `QosEgressMapUpdate` + `QosMarkEnableDisable`; first impl skipped the record step so mark read an uncaptured DSCP.
- **filter mark (L205)** - VPP classify matches header bytes not SKB metadata; needs a VPP-native metadata primitive.
- **qdisc prio -> QoS egress map (L206)** - needs an explicit operator-facing DSCP->class binding; ~~`egressMapFromPrioClasses` retained as skeleton~~ (design correction: the symbol exists in no Go file and never landed under internal/ - it was a review-rejected draft; work item starts from zero code, design history in `plan/learned/627-fw-7-traffic-vpp.md:43-60`).
- **Multi-class HTB/TBF shaping (L210)** - per-class shaping needs filter-based steering to distinct policers (depends on the classify pipeline above); single-class works today.

### Design-time corrections (2026-07-09, verified with file:line)

| Triage claim | Reality today |
|--------------|---------------|
| `egressMapFromPrioClasses` retained as skeleton | Zero hits in any Go file; never committed under internal/ (`git log -S` empty). Fresh implementation required |
| Verifier rejections at :15-17, :142-183 | Error vars still :15-17 (verified firsthand); sites drifted to `verifyQdiscType` :148-153 and `verifyFilter` :176-192; multi-class reject at :97-100 |
| Pipeline must be designed from scratch | **Major in-repo prior art exists**: the firewall VPP backend already implements table create + session + interface attach + per-policer steering (`firewall/vpp/classify_linux.go`: classifyMaskMatch :30-63, applyTermClassify :118-187 with `policerClassifySetInterface` :170 and `classifySetInterfaceIPTable` :179 - verified firsthand); the iface VPP backend already implements the full 3-step QoS pipeline for VLAN PCP (`ifacevpp.go` enableVLANQoS :334-387, verified firsthand - includes the record step the first traffic impl skipped) |
| (implicit) mark has no VPP primitive | `ClassifyAction` enum includes `SET_METADATA=3` and `ClassifyAddDelSession` carries `Metadata`/`OpaqueIndex` fields (vendored classify.ba.go:35-41,:154-164) - a native metadata primitive exists |
| verify.go comments point operators at deferral rows | `verify.go:53-54,:98,:175` reference `plan/deferrals.md` rows that were migrated into this spec at triage - retarget those comments when touched |

## Required Reading

### Source files / docs

- [ ] `internal/plugins/traffic/vpp/verify.go`
  → Constraint: exact-or-reject is the contract (`ai/rationale/exact-or-reject.md`) - a feature flips from reject to accept only when fully wired; partial acceptance is banned
  → Constraint: policer name "ze/<iface>/<class>" ≤ 64 bytes (maxPolicerNameLen :25) applies to every new per-class policer
- [ ] `internal/plugins/traffic/vpp/backend_linux.go`, `ops.go`
  → Constraint: extension seam = add methods to the `vppOps` interface + `govppOps` + `fakeOps` (ops.go:26-36 documents this); undo-stack pattern for CREATE, rebind for UPDATE (applyInterface :287-352); orphan cleanup must learn new object kinds (cleanupStartupOrphans :187-220, reconcileRemovals :379-407)
- [ ] `internal/plugins/firewall/vpp/classify_linux.go` + `firewall/vpp/backend_linux.go` (:637-712 ops wrappers)
  → Decision: REUSE this pipeline shape for filter protocol: mask/match builder (IPv4 protocol at byte 9, skip_n_vectors=0), `ClassifyAddDelTable` → `ClassifyAddDelSession` → `ClassifySetInterfaceIPTable`; per-policer steering via `PolicerClassifySetInterface`
  → Constraint: firewall's builder is IPv4-only (applyPrefixMaskMatch bails on !Is4); IPv6 next-header (offset per IPv6 header, next-header byte 6) is NEW work in this spec
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` (enableVLANQoS :334-387, UpdateVLANQoSMap :280-328)
  → Decision: REUSE the 3-step order for filter dscp: QosRecordEnableDisable (ingress, the step the first impl skipped) → QosEgressMapUpdate → QosMarkEnableDisable, with `QOS_API_SOURCE_IP` instead of SOURCE_VLAN
  → Constraint: VLAN precedent enforces identity ingress map (:247-251) - decide and document whether IP-source record has the same constraint
- [ ] `internal/plugins/traffic/netlink/translate_linux.go` (translateFilter :125-152, dscpFilters :154-169, protocolFilters :171-185)
  → Constraint: behavior parity target - IPv4 TOS bits dscp<<18 mask 0x00FC0000; IPv6 dscp<<22 mask 0x0FC00000; protocol IPv4 byte at off 8 (u32 word), IPv6 next-header at off 4; mark on netlink is SKB FwFilter - VPP mark is NOT SKB parity (see Key Design Decisions)
- [ ] `internal/component/traffic/yang/ze-traffic-control-conf.yang` (filter-type :44-51, class list :95-120, match list :122-136) + `internal/component/traffic/model.go` (:102-108, :170-171)
  → Constraint: config surface already exists for all five items (match type mark/dscp/protocol with string value; class priority uint8); NO new YANG leaves expected except possibly an explicit dscp→class binding refinement - prefer reusing per-class `match { type dscp }` entries as the binding (L206)
- [ ] `vendor/go.fd.io/govpp/binapi/classify/` + `binapi/qos/`
  → Constraint: available RPCs: ClassifyAddDelTable/AddDelSession/SetInterfaceIPTable, PolicerClassifySetInterface, InputACL/OutputACLSetInterface; QosRecord/EgressMap/Mark/StoreEnableDisable; ClassifyAction SET_METADATA=3
- [ ] `scripts/evidence/effective-vpp.py` (run_traffic_evidence :550-620), `test/scripts/vpp_stub.py`, `test/traffic/*.ci`
  → Constraint: three test tiers exist: fakeOps unit seam, vpp_stub .ci, real-VPP Docker evidence (vppctl assertions) - every new pipeline needs all three
  → Constraint: `test/traffic/020-vpp-reject-dscp-filter.ci` pins the dscp REJECTION and `011-vpp-reject-hfsc.ci` pins qdisc rejection - implementing the features requires inverting/replacing those tests deliberately (never weakening: the new tests must assert programming, not just absence of rejection)

**Key insights:**
- This spec is mostly assembly of two proven in-repo pipelines (firewall classify, iface 3-step QoS) behind the traffic backend's existing ops seam.
- The two historical failure modes are named and must become tests: (1) classify table created but never attached to an interface; (2) QoS mark without record.
- IPv6 next-header matching is the only genuinely new wire-level construct.
- Exact-or-reject means the verifier and the backend flip together per feature, never ahead of each other.

## Current Behavior (MANDATORY)

**Source files read (2026-07-09):**

- [ ] `internal/plugins/traffic/vpp/verify.go` - rejects all filters (:15-17 errors, verifyFilter :176-192, verified firsthand), all qdiscs except single-class HTB/TBF (:97-100, :148-153)
- [ ] `internal/plugins/traffic/vpp/translate.go` - policer translation only (policerFromClass :94-150); no filter/prio translation
- [ ] `internal/plugins/traffic/vpp/backend_linux.go` - Apply → applyWithOps → per-interface policer create + egress bind (applyInterface :287-352); six wrapped RPCs (govppOps :455-559)
- [ ] `internal/plugins/firewall/vpp/classify_linux.go` - working classify pipeline incl. interface attach + policer steering (verified firsthand :118,:170,:179)
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - working 3-step VLAN QoS (verified firsthand :334-336)

**Behavior to preserve:**
- Single-class HTB/TBF policer programming, orphan cleanup, undo-on-error, reconcileRemovals semantics.
- Exact-or-reject for anything NOT implemented by this spec (e.g. hfsc/netem qdiscs stay rejected).
- Netlink backend behavior untouched.
- Firewall and iface VPP pipelines untouched (reuse by pattern or shared helper extraction, no behavior change).

**Behavior to change:**
- Verifier accepts filter protocol / dscp / mark and multi-class HTB/TBF (with per-class steering) under vpp.
- Backend programs classify tables/sessions/attachments, QoS record+map+mark, per-class policer steering, prio→egress-map binding.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `traffic-control` YANG under the `vpp` backend; verifier gate at OnConfigVerify; Apply at OnConfigure

### Transformation Path
1. Config (interfaces/classes/matches) resolves to `traffic.Interface` model
2. `Verify` accepts implemented shapes (post-spec) or rejects exactly
3. `translate.go` produces VPP objects: policers (existing), classify mask/match + sessions (new), egress-map rows (new)
4. `applyWithOps` programs VPP via ops seam: tables → sessions → interface attach → QoS record/map/mark → policer steering; undo stack on error
5. Kernel/VPP state observable via vppctl (evidence) and stub JSONL (functional)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config → trafficvpp | YANG resolution + verifier | [ ] |
| trafficvpp → VPP | govpp classify/qos/policer binary API via vppOps seam | [ ] |
| tests → VPP | fakeOps (unit) / vpp_stub.py (functional) / Docker vppctl (evidence) | [ ] |

### Integration Points
- `internal/plugins/traffic/vpp/` (verify, translate, backend, ops)
- vendored govpp classify/qos binapi (already present)
- `scripts/evidence/effective-vpp.py` (extend run_traffic_evidence)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate; consider extracting a shared classify mask/match helper if firewall + traffic duplication exceeds two call sites - `abstract at 2+ use cases`)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Corrected evidence holds at implement time | re-verified 2026-07-09 (firsthand: verify.go:15-17, classify_linux.go:118/:170/:179, ifacevpp.go:334-336, egressMapFromPrioClasses absent) | Re-scope item | grep/LSP at implement-audit | confirmed |
| A-4 | IPv6 next-header mask/match expressible with skip/match vectors | CONFIRMED real VPP v25.10: ze programs the ip6 classify table (next-header at absolute frame byte 20) and binds ip6-policer-classify; evidence run green | IPv6 protocol filtering stays rejected | phase 1 real-VPP spike + evidence (done) | **confirmed** |
| A-STEER | Classify session steering to a policer via binary API | CONFIRMED real VPP: `ClassifyAddDelSession.HitNextIndex = policer index` (action=0) is byte-for-byte VPP's own `policer-hit-next`. GOTCHA: the match must span skip+match vectors, so the traffic tables use skip=0 + full-width absolute-offset mask (IPv4 proto byte 23, IPv6 nh byte 20); a skip=1 short match returns INVALID_VALUE(-7). Packet-level policing not injected (loopback pg punts); state-level programming byte-matches VPP's canonical form | Keep protocol rejected | phase 1 spike + evidence (done) | **confirmed (programming)** |
| A-6 | vpp_stub asserts new RPCs without material stub work | BROKEN: the stub has no `sw_interface_dump`/`policer_add_del` handlers, so a full traffic Apply cannot run against it (only `classify_add_del_table` was added here). Apply-level stub `.ci` needs stub work owned by `plan/spec-finish-vpp-stub.md` | rely on unit (fakeOps) + real-VPP evidence tiers | phase 1 stub attempt (done) | **broken (coordinate)** |
| A-2 | `match dscp` under vpp = police-by-dscp: classify the DSCP/TOS bits at absolute frame offsets and steer to the class policer (same pipeline as `filter protocol`) | netlink parity target (translate_linux.go); classify pipeline proven for protocol (1096) | Keep dscp rejected | real-VPP spike (offsets) + real-VPP evidence (run_traffic_dscp_evidence) | **CONFIRMED (USER decision 2026-07-10 = police-by-dscp).** The original record/map/mark reading is SUPERSEDED: the spike proved it only REMARKS, it cannot police DSCP-matched traffic. Real VPP v25.10 accepts the dscp classify masks (ip4 TOS byte 15 mask 0xFC; ip6 TC bytes 14/15 masks 0x0F/0xC0) and the evidence run programs tables bound + steering + reconcile removal. |
| A-3 | `SET_METADATA` classify action + policer-classify steering give filter mark a faithful-enough semantic (mark = steering key, not SKB parity) | classify.ba.go action enum + firewall steering precedent | Keep mark rejected with a better error naming the semantic gap; record deferral | phase 3 spike on real VPP | **BROKEN (kept rejected)**: real-VPP v25.10 spike (Docker ligato/vpp-base) -- `classify session` CLI exposes only `set-ip4-fib-id`/`set-ip6-fib-id`/`set-sr-policy-index`, NO set-metadata; the binary SET_METADATA/opaque_index stores an opaque value consumed only by specific downstream graph nodes (ACL, SR-policy), with no feature arc that reads it back into the packet like Linux SKB fwmark persists. No faithful mark semantic exists. AC-3 fallback applied: mark stays rejected with an error naming the gap (verify.go errFilterMarkNotSupportedByBackend), deferral tracked here. |
| A-4 | IPv6 next-header mask/match at the ip-ACL arc is expressible with skip/match vectors like IPv4 | classify table model (mask vectors); netlink parity values known | IPv6 protocol filtering stays rejected (exact-or-reject) with deferral destination | phase 1 stub + evidence test | unvalidated |
| A-5 | Existing per-class `match { type dscp }` entries suffice as the operator-facing DSCP→class binding for prio (no new YANG) | YANG match list :122-136 already typed | Add a scoped YANG leaf per `ai/patterns/config-option.md` | phase 4 design review | **MOOT (superseded).** AC-4 resolved as prio-rejection-retained (USER 2026-07-10); no prio->dscp binding is built, so this assumption no longer applies. `egressMapFromPrioClasses` is not implemented. |
| A-CHAIN | Multi-class steering needs `ClassifyAddDelTable.NextTableIndex` chaining only for mixed-field masks; same-field multi-class uses one table with N sessions | real-VPP spike + classify model | Reject mixed-field-in-one-interface | real-VPP spike (chain fall-through) | **CONFIRMED**: spike read back a chained head with `NextTbl = successor`; multi-session-per-table steers to distinct policers (`next_index` 40/41). Evidence run (multi-class protocol) green. |
| A-6 | vpp_stub.py can assert the new RPC names without material stub work | stub scrapes (name,crc) from vendored binapi | Extend stub (open spec `plan/spec-finish-vpp-stub.md` exists - coordinate, don't duplicate) | run stub .ci during phase 1 | **BROKEN (unchanged, documented).** Apply-tier `.ci` cannot run a full traffic Apply against the stub (no sw_interface_dump/policer_add_del). Validation stays unit (fakeOps) + real-VPP evidence; verify-tier `.ci` used for accept/reject pins. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Repeat of historical failure 1: table exists but not attached | evidence shows classify table with no interface binding | Wiring test asserts `ClassifySetInterfaceIPTable` called (fakeOps) AND vppctl shows binding (evidence) |
| R-2 | Repeat of historical failure 2: mark without record | dscp filter passes unit tests but no remark on wire | Pipeline order enforced in one function; evidence asserts qos record state on ingress interface |
| R-3 | Wrong match offset (historical: "matched wrong offset") | classify session never hits | Golden mask/match vectors unit-tested against netlink parity values; real-VPP evidence sends a probe packet where feasible |
| R-4 | Orphan cleanup misses new object kinds (tables/sessions/qos state) leak across restarts | evidence restart phase shows stale tables | Extend cleanupStartupOrphans + reconcileRemovals per object kind; restart assertions in evidence run |
| R-5 | Multi-class policer names collide/truncate | verifier name checks pass but VPP upserts | Existing 64-byte verify check covers per-class names; add boundary test |
| R-6 | Inverting 020/011 .ci weakens regression coverage for still-rejected shapes | rejected-shape coverage drops | Keep rejection tests for every still-rejected shape (hfsc, netem, IPv6-mark combos not implemented) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `filter protocol` (IPv4+IPv6) under vpp backend applied | → | table+session created, `ClassifySetInterfaceIPTable` attached | `TestApplyFilterProtocolAttaches` (fakeOps) + `.ci` `test/traffic/030-vpp-filter-protocol.ci` (stub) |
| `filter dscp` under vpp | → | record→map→mark programmed in order | `TestApplyFilterDscpThreeStep` + `.ci` `031-vpp-filter-dscp.ci` |
| `filter mark` under vpp | → | SET_METADATA session + steering | `TestApplyFilterMarkMetadata` + `.ci` `032-vpp-filter-mark.ci` |
| prio qdisc with per-class dscp matches | → | egressMapFromPrioClasses → QosEgressMapUpdate | `TestEgressMapFromPrioClasses` + `.ci` `033-vpp-prio-egress-map.ci` |
| Multi-class HTB with per-class filters | → | per-class policers + PolicerClassifySetInterface steering | `TestApplyMultiClassSteering` + `.ci` `034-vpp-multiclass-htb.ci` |
| Real VPP (Docker evidence) | → | vppctl shows tables bound, qos record/mark, per-class policers | `scripts/evidence/effective-vpp.py` extended `run_traffic_evidence` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `filter protocol tcp` (and an IPv6 case) on a vpp interface | Verifier accepts; classify table with correct IPv4 proto-byte-9 / IPv6 next-header mask created; session added; table attached to the interface (ip-ACL arc); undo removes all three on error; orphan cleanup covers tables |
| AC-2 | `filter dscp <name>` | (SUPERSEDED 2026-07-10 by USER decision: POLICE-BY-DSCP, not record/map/mark.) Verifier accepts; per-family classify tables with the DSCP/TOS mask (IPv4 TOS byte 15 mask 0xFC; IPv6 traffic-class bytes 14/15 masks 0x0F/0xC0) created; session per dscp value steering to the class policer; table bound to the interface policer-classify feature; undo removes all on error; reconcile tears down on removal - the SAME pipeline as `filter protocol` |
| AC-3 | `filter mark 0xN` | Verifier accepts; classify session with Action=SET_METADATA, Metadata=mark; documented semantics: VPP metadata steering, not Linux SKB-mark parity (or, if A-3 breaks: rejection retained with an error naming the semantic gap + recorded deferral destination) |
| AC-4 | `qdisc prio` under vpp | (RESOLVED 2026-07-10 by USER decision: REJECTION-RETAINED, no scheduler→remark substitution.) Verifier REJECTS prio with an actionable, evidence-backed error naming the semantic gap (netlink prio is a priority scheduler; VPP exposes no prio scheduler); `egressMapFromPrioClasses` is NOT built; rejection pinned by unit test + rejection `.ci` |
| AC-5 | HTB with 2+ classes, each with a filter | Verifier accepts; per-class policers created (name limit enforced), classify steering binds traffic to the right policer (`PolicerClassifySetInterface`); single-class behavior byte-identical to today |
| AC-6 | Still-unimplemented shapes (hfsc, netem, etc.) | Remain exactly-rejected; rejection `.ci` coverage retained (020/011 replaced by new rejection tests for still-rejected shapes + acceptance tests for implemented ones) |
| AC-7 | Real-VPP evidence run (`make ze-deployment-vpp-test`) | Extended assertions pass: classify table bound to interface, qos record enabled, egress map present, per-class policers exist; restart + orphan-cleanup phases cover the new object kinds |
| AC-8 | `verify.go` stale comments | Deferral-row pointers (:53-54,:98,:175) retargeted to this spec's learned summary |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator polices TCP traffic on a VPP NIC | config → verifier → classify table+session+attach → policer steering | `030-vpp-filter-protocol.ci` + evidence |
| 2 | Operator remarks DSCP on egress | config → 3-step QoS pipeline | `031-vpp-filter-dscp.ci` + evidence |
| 3 | Operator ports a linux tc prio setup to VPP | config → egress-map binding | `033-vpp-prio-egress-map.ci` |
| 4 | Operator shapes two service classes at different rates | config → multi-class policers + steering | `034-vpp-multiclass-htb.ci` + evidence |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestClassifyMaskMatchIPv4Protocol`, `TestClassifyMaskMatchIPv6NextHeader` | `internal/plugins/traffic/vpp/translate_test.go` | AC-1 golden vectors (R-3) | |
| `TestApplyFilterProtocolAttaches`, `TestApplyFilterProtocolUndo` | `apply_test.go` (fakeOps) | AC-1, R-1 | |
| `TestApplyFilterDscpThreeStep`, `TestDscpRemovalDisablesAll` | `apply_test.go` | AC-2, R-2 | |
| `TestApplyFilterMarkMetadata` | `apply_test.go` | AC-3 | |
| `TestEgressMapFromPrioClasses` (incl. no-match rejection) | `translate_test.go` | AC-4 | |
| `TestApplyMultiClassSteering`, `TestPolicerNameBoundaryMultiClass` | `apply_test.go`, `verify_test.go` | AC-5, R-5 | |
| `TestVerifyAcceptsImplementedShapes`, `TestVerifyStillRejectsUnimplemented` | `verify_test.go` | AC-6 | |
| `TestOrphanCleanupClassifyTables` | `apply_test.go` | R-4 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| dscp value | 0-63 | 63 | N/A | reject at parse (existing named-dscp path, 021 precedent) |
| mark value | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A | uint32 model bound |
| policer name len (multi-class) | ≤64 | 64 | - | verify reject (existing :25 check) |
| class count per interface | 1-N (VPP policer/session budget) | document tested max | - | document behavior |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `030-vpp-filter-protocol.ci`, `031-vpp-filter-dscp.ci`, `032-vpp-filter-mark.ci`, `033-vpp-prio-egress-map.ci`, `034-vpp-multiclass-htb.ci` | test/traffic (vpp_stub) | each feature programs expected RPCs | |
| replacement rejection tests for still-rejected shapes | test/traffic | exact-or-reject preserved | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Real-VPP evidence (extended run_traffic_evidence) | scripts/evidence/effective-vpp.py (Docker ligato/vpp-base) | real VPP daemon | programming accepted by a real VPP, survives restart | |

## Files to Modify

- `internal/plugins/traffic/vpp/verify.go` - accept implemented shapes; retarget stale comments
- `internal/plugins/traffic/vpp/translate.go` - classify mask/match builders, egress-map builder, per-class steering translation
- `internal/plugins/traffic/vpp/backend_linux.go` + `ops.go` - new vppOps methods (classify table/session/attach, qos record/map/mark, policer-classify), apply/undo/orphan integration; fakeOps mirrors
- `test/traffic/011-vpp-reject-hfsc.ci`, `020-vpp-reject-dscp-filter.ci` - replaced per AC-6
- `scripts/evidence/effective-vpp.py` - extend run_traffic_evidence
- `docs/` per Documentation Update Checklist (features.md traffic rows, comparison.md if VPP QoS parity is a comparison line)

## Files to Create

- `test/traffic/030..034-*.ci` as listed
- (possibly) shared classify helper if extraction from firewall/vpp is warranted at 2+ users - decide at implement time, record in Key Design Decisions

## Implementation Steps

1. **Phase: Wiring (filter protocol IPv4)** - ops methods + failing fakeOps wiring test asserting attach (R-1 killer); verifier flip for protocol-IPv4 only.
2. **Phase: IPv6 next-header** - golden vectors → implementation → A-4 verdict.
3. **Phase: filter dscp 3-step** - ordered pipeline, removal path (R-2 killer).
4. **Phase: prio → egress map** - `egressMapFromPrioClasses` fresh implementation (A-5).
5. **Phase: filter mark** - SET_METADATA spike on real VPP first (A-3), then implement or re-reject with documented gap.
6. **Phase: multi-class steering** - per-class policers + PolicerClassifySetInterface; lift :97-100 restriction.
7. **Phase: evidence + docs** - extend effective-vpp.py; replace rejection .cis; doc rows (AC-6..AC-8).
8. **Full verification** - `make ze-verify`; `ze-traffic-test`; `ze-deployment-vpp-test` where Docker available.
9. **Complete spec** - audit tables, `plan/learned/NNN-followup-vpp-traffic.md`, two-commit closure.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

Pre-checks: `audit-test-relaxation.py` CLEAN (no deleted/weakened tests). `make
ze-validate`: 3 issues, ALL in `internal/component/iface/tunnel.go` (the
concurrent iface session's files) -- none in this spec's surface.

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Two classes with the IDENTICAL steering filter (both `match protocol tcp`) pass verify but their classify sessions collide on one match key -> VPP keeps the last, silently leaving one class's policer dead (exact-or-reject violation) | verify.go verifyInterface | FIXED: added `verifyNoDuplicateSteering` rejecting the same (type,value) across classes + `TestVerifyRejectsDuplicateSteeringAcrossClasses` |
| 2 | NOTE | Duplicate class names / two matches of one type in a class would collide | YANG | Not reachable: `list class { key name }` + `list match { key type }` reject at parse; no code guard needed |
| 3 | NOTE | Chain ordering for a packet matching sessions in two chained (mixed-field) tables = deterministic-but-arbitrary priority (head wins; head = mask-sorted first) | classify_linux.go groupSteeringsByMask | Acknowledged; documented in Known Limitations (netlink has the same all-priority-1 ambiguity) |
| 4 | NOTE | Same-process re-apply with a DIFFERENT filtered-class set on real VPP relies on `PolicerClassifySetInterface(isAdd=true)` repointing the head (matches shipped protocol behavior; unit-tested; evidence covers apply + drop-to-empty, not apply->apply-different) | classify_linux.go applyInterfaceClassify | Acknowledged; documented in Known Limitations |

### Fixes applied
- `verify.go`: added `verifyNoDuplicateSteering` + `steerKey`; rejects the same
  steering (type,value) on two classes of one interface. Test:
  `TestVerifyRejectsDuplicateSteeringAcrossClasses` (verify_test.go).

### Run 2 (re-run after fix)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | (none above NOTE) | wiring verified (all new symbols reach `applyInterface`/`Verify`); allocation bounded (fixed 32-byte masks); no hot-path sprintf; no new runtime dependency (no doctor check needed); no RFC/protocol wire code | - | clean |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Every work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reuse firewall classify pipeline shape via the traffic ops seam | Design classify handling from scratch | applyTermClassify is proven on real VPP incl. interface attach + policer steering - the two historical failure modes are already solved there |
| **(2026-07-10, USER, binding) `match dscp` under vpp = POLICE-BY-DSCP: classify the DSCP/TOS bits (IPv4 TOS byte, IPv6 traffic-class) at absolute frame offsets and steer to the class policer - the SAME pipeline as the shipped `filter protocol`.** SUPERSEDES the original "DSCP via 3-step record+map+mark" decision (struck below). | (B) 3-step record/map/mark (QosRecordEnableDisable(SOURCE_IP) + QosEgressMapUpdate + QosMarkEnableDisable) | A-2 real-VPP spike proved record/map/mark is a REMARKING primitive: it rewrites egress DSCP but CANNOT police DSCP-matched traffic. The spec's own parity requirement (Required Reading, translate_linux.go row) says `match dscp` should police matched traffic like `match protocol`. Police-by-dscp is coherent, sibling-consistent with `match protocol`, and reuses the real-VPP-validated classify pipeline. Real VPP v25.10 validated the TOS/TC offsets and steering. |
| ~~DSCP via 3-step record+map+mark (SOURCE_IP)~~ STRUCK 2026-07-10 | - | Real-VPP spike (A-2) proved record/map/mark cannot police DSCP-matched traffic; only remarks. Superseded by police-by-dscp above. `egressMapFromPrioClasses` / QoS record/map/mark are NOT built. |
| **(2026-07-10, USER, binding) `qdisc prio` under vpp STAYS REJECTED** - no scheduler→remark substitution. AC-4 resolves as rejection-retained with evidence-backed error text. `egressMapFromPrioClasses` is NOT built. | (A) map prio→DSCP egress map (a semantic substitution: netlink prio is a priority SCHEDULER, VPP has no prio scheduler); (B) reject | These VPP APIs expose no prio scheduler. Mapping prio to a DSCP egress map is a silent semantic substitution that exact-or-reject forbids without an explicit operator-facing contract. Rejection with an actionable error is the honest posture; no L206 deferral row exists in plan/deferrals.md (migrated into this spec at triage), so the resolution is recorded here only. |
| Mark = rejected (Linux SKB fwmark has no faithful VPP equivalent) | SET_METADATA steering semantic | A-3 real-VPP spike: classify SET_METADATA stores opaque graph-node metadata, not a persistent packet mark. No faithful semantic exists; rejection with an evidence-backed error is honest. |
| Multi-class steering: group per-interface per-family classify sessions by MASK (field type); one table per distinct mask with N sessions (each session's HitNextIndex = its class policer); chain distinct-mask tables via `ClassifyAddDelTable.NextTableIndex` | One table per class chained (prior-session framing); one combined-mask table | Same-field multi-class (the common AC-5 case: two classes on different protocols) needs only ONE table per family with N sessions - no chaining. Chaining is reserved for the mixed-field case (a class/interface mixing protocol and dscp filters), which needs distinct masks. Real-VPP v25.10 validated multi-session steering AND NextTableIndex chain fall-through. |
| Verifier flips per-feature with acceptance+rejection test pairs | Flip everything at once | exact-or-reject contract; keeps unimplemented shapes pinned (R-6) |

## Known Limitations

- `filter mark` is rejected: Linux SKB fwmark has no faithful VPP equivalent (AC-3, real-VPP-validated).
- `qdisc prio` is rejected: VPP exposes no priority scheduler; no scheduler->DSCP-remap substitution is made (AC-4, USER decision 2026-07-10). Use htb/tbf with per-class protocol/dscp filters.
- Multi-class under vpp requires EVERY class to carry a steering filter; a mix of one unfiltered "default" class with filtered classes is rejected (an unfiltered class binds to the egress output arc, a different direction/feature from the ingress classify path, and two unfiltered classes would stack producing min(rates)).
- Two classes may not select the identical steering match (same protocol/dscp value): rejected at verify (classify sessions are keyed by match; a duplicate would silently last-win).
- Overlapping-but-distinct filters (a packet that matches sessions in two chained mixed-field tables, e.g. both `protocol tcp` and `dscp 46` on different classes) resolve by chain order = mask-sorted, head wins. This is a deterministic-but-arbitrary priority; netlink has the same all-priority-1 ambiguity. Distinct classes on distinct fields for the same packet is an unusual config.
- Same-process re-apply that changes the filtered-class set relies on `PolicerClassifySetInterface(isAdd=true)` repointing the bound head (the shipped protocol-filter behavior); real-VPP evidence covers apply + reconcile-to-empty, unit tests cover drop-one-class reconcile.
- VPP mark filters do not interoperate with Linux SKB-mark consumers; the semantic is steering within VPP (documented at AC-3).
- Classify pipeline applies to the ip-ACL / policer-classify arc; L2-only interfaces are not handled (as today).
- Apply-tier `.ci` (programs-expected-RPCs against the stub) is blocked on A-6 (vpp_stub lacks sw_interface_dump/policer_add_del, owned by `plan/spec-finish-vpp-stub.md`). Validation is unit (fakeOps) + real-VPP evidence; verify-tier `.ci` pins accept/reject.
- Real-VPP packet-level probes limited by the Docker evidence environment (loopback pg punts); state-level vppctl assertions are the contract.

## Notes
- Designed 2026-07-09 from skeleton; user instruction 2026-07-09 authorized batch conversion to ready.
- Coordinate with open `plan/spec-finish-vpp-stub.md` before extending vpp_stub.py (A-6) - don't duplicate.

## Progress Log

### 2026-07-09 -- Phases 1-2 landed (filter protocol IPv4 + IPv6), real-VPP validated
Implemented and committed the classify + policer-classify pipeline for
`filter protocol` (both families). Verifier flips protocol reject->accept
(AC-1); DSCP, mark, and multi-class stay rejected (AC-6 preserved).

- **Code**: `translate.go` (`protocolClassifyVectors` golden builder), `ops.go`
  (classify seam), `backend_linux.go` (govppOps wrappers + apply branch),
  `classify_linux.go` (NEW: create/undo/reconcile), `verify.go` (protocol
  accepted, bound-checked), `binapi_imports.go` (classify anchor).
- **Tests**: golden vectors + apply-attaches (R-1 killer) + undo + reconcile
  teardown (fakeOps) + verify accept/reject; all green, `ze-lint-changed` clean.
- **Real-VPP evidence** (`effective-vpp.py run_traffic_protocol_evidence`):
  classify tables created and BOUND to the interface policer-classify feature,
  steering to the class policer, removed on reconcile. Passes on VPP v25.10.

**Key finding (see A-STEER)**: VPP's policer-classify session steering is
`ClassifyAddDelSession.HitNextIndex = policer index` (action=0), byte-for-byte
its own `policer-hit-next`. The match must span skip+match vectors, so tables
use skip=0 with a full-width absolute-offset mask (IPv4 proto byte 23, IPv6
next-header byte 20). A skip=1 short match returns INVALID_VALUE(-7). This
ground truth is the foundation for the remaining phases (dscp/prio/mark/multi).

**Two pre-existing evidence-script bugs fixed** (blocked ALL VPP evidence):
`ensure_linux_binaries` built the nonexistent `./cmd/ze-test` (now
`-tags ze_test ./cmd/ze`), and the traffic configs used the retired top-level
keyword `traffic-control {` (now `traffic { control { ... } }`). With these,
FIB + MPLS + traffic + protocol evidence phases all pass; the firewall phase
still fails on a SEPARATE pre-existing schema drift (`unknown field in from:
connection`) that is out of this spec's scope.

**Remaining (open)**: phase 3 dscp 3-step, phase 4 prio->egress map, phase 5
mark (A-3 spike), phase 6 multi-class steering (verify.go:97-100), phase 7
evidence/docs for those + `.ci` acceptance (A-6 stub work in
`spec-finish-vpp-stub.md`). Stub apply-tier tests remain blocked on A-6.

### 2026-07-09 -- Phase 5 (mark) resolved; phases 3/4/6 BLOCKED on design decisions

Real-VPP spikes run on Docker `ligato/vpp-base` (VPP v25.10). Findings:

**Phase 5 / AC-3 (filter mark): DONE via the designed fallback (mark stays
rejected).** A-3 BROKE on real VPP: `classify session` exposes only
`set-ip4-fib-id`/`set-ip6-fib-id`/`set-sr-policy-index` -- there is NO
set-metadata CLI action, and the binary `SET_METADATA`/`opaque_index` stores an
opaque value consumed only by specific downstream graph nodes (ACL, SR-policy),
with no feature arc that reads it back into the packet the way Linux SKB fwmark
persists. No faithful mark semantic exists. Per AC-3's fallback, the rejection
is retained with an error naming the gap (`verify.go`
`errFilterMarkNotSupportedByBackend`, now evidence-backed) and the deferral is
recorded here. This AC is satisfied.

**Phases 3 (dscp) and 4 (prio): BLOCKED -- the spec is internally inconsistent
about what `match dscp` MEANS under vpp, and this is a user decision.**

The config surface `class { rate 10mbit; match dscp { value 48 } }` has a RATE
(a policer) and a MATCH. Two readings, mutually exclusive, both present in the
spec:

- (A) **Parity / police-by-dscp** -- the spec's Required Reading (translate_linux.go
  row) states dscp *behavior parity* is the target and gives the exact bit
  patterns; netlink's `match dscp` steers matched packets to the class policer,
  identical to the already-shipped `match protocol`. This reading = CLASSIFY the
  DSCP/TOS bits and steer to the policer (reuse the proven phase 1-2 pipeline; ip4
  TOS at absolute byte 15 mask 0xFC, ip6 TC across bytes 14/15 masks 0x0F/0xC0).
- (B) **Remark / record-map-mark** -- AC-2 and the Key Design Decisions say
  `QosRecordEnableDisable(SOURCE_IP)` + `QosEgressMapUpdate` + `QosMarkEnableDisable`.
  Real VPP CONFIRMS this mechanism programs (A-2: `qos record loop0 ip` +
  `qos mark loop0 map <id> ip` accepted, state present) -- BUT it is a REMARKING
  primitive: it rewrites egress DSCP, it does NOT police DSCP-matched traffic.
  It also has no output-DSCP source in the config (`match dscp { value X }` gives
  one value; a record/map/mark table needs an input->output map), so the map
  contents for a single value are under-specified.

Reading (B) cannot police the matched traffic, so a `class` with a `rate` +
`match dscp` would parse and "apply" but not actually police -- exactly the
silent-approximation `ai/rationale/exact-or-reject.md` forbids. Reading (A) is
coherent, achieves the stated parity, is sibling-consistent with `match protocol`,
and reuses proven real-VPP-validated infrastructure.

**Recommendation: implement `match dscp` as reading (A) -- classify the DSCP bits
and steer to the class policer** (correct AC-2's mechanism note, keep the parity
goal). If the operator genuinely wants VPP DSCP *remarking* (reading B), that is a
different feature that needs its own config surface (an explicit output DSCP, not a
`match`), and the YANG leaf description must document the divergence per
exact-or-reject. This choice was NOT made unilaterally because it contradicts the
explicit "record/map/mark" instruction; it needs user confirmation.

**Phase 4 (prio -> egress map)** inherits the same question and adds a second
divergence: `qdisc prio` is a priority *scheduler* in netlink, but these VPP APIs
expose no prio scheduler -- mapping prio to a DSCP egress map is a semantic
substitution that must be made the documented contract (YANG description) per
exact-or-reject, or rejected. `egressMapFromPrioClasses` per A-5 (DSCP-input ->
class-priority-output, one map pushed per interface) is implementable once the
dscp semantic above is settled and the prio->remap divergence is accepted +
documented.

**Phase 6 (multi-class HTB/TBF steering): implementable, uncontested semantic,
but genuinely NEW wire work.** Police-by-protocol across N classes = clean netlink
parity, no divergence. BUT `PolicerClassifySetInterface` binds ONE table per
family per interface, so N per-class tables must be CHAINED via
`ClassifyAddDelTable.NextTableIndex` (head bound, miss -> next). The firewall prior
art does NOT chain (it rebinds per-term, last-table-wins -- a known unvalidated
limitation, see plan/learned/1096), so chaining is new and needs its own real-VPP
validation plus an N-table reconcile/undo/orphan rework of `classify_linux.go`
(currently a single `classifyBinding` per interface). Recommended as the next
phase once phase 3's semantic is settled.

**Session outcome (honest stop):** AC-3 resolved (mark, real-VPP-backed). AC-2
(dscp), AC-4 (prio), AC-5 (multi-class) are BLOCKED pending the user's decision on
the `match dscp` semantic (reading A vs B) and acceptance of the prio->remap
divergence. Committing this progress; spec stays open. No divergent/incoherent
feature was shipped against exact-or-reject.

### 2026-07-10 -- USER decisions applied; phases 3 + 6 landed, AC-4 resolved (spec CLOSED)

USER decisions (binding, recorded in Key Design Decisions):
1. `match dscp` under vpp = **POLICE-BY-DSCP** (reading A): classify the DSCP/TOS
   bits and steer to the class policer, same pipeline as `filter protocol`. The
   original AC-2 "record/map/mark" reading is SUPERSEDED (struck).
2. `qdisc prio` under vpp **STAYS REJECTED** (no scheduler->remark substitution).
   AC-4 resolves as rejection-retained with an evidence-backed error;
   `egressMapFromPrioClasses` is NOT built. No L206 deferral row exists in
   plan/deferrals.md (migrated into this spec at triage), so recorded here only.

**Real-VPP spike (Docker ligato/vpp-base, VPP v25.10)** de-risked the design
before wiring (tmp/vpp-spike.py; results in the Progress Log evidence):
- IPv4 DSCP mask accepted at absolute byte 15 (0xFC); IPv6 across bytes 14/15
  (0x0F/0xC0). Offsets confirmed.
- Multi-session per table: one table, two sessions, distinct `next_index` (40/41).
- `ClassifyAddDelTable.NextTableIndex` chaining: head table read back with
  `NextTbl = successor index` (clean fall-through linkage).

**Phase 3 (dscp police-by-dscp), Phase 6 (multi-class steering):** implemented.
`dscpClassifyVectors` (ip4 TOS byte 15 / ip6 TC bytes 14-15); a per-interface,
per-family classify model that groups steerings by MASK (one table per distinct
mask, a session per class -> its own policer) and chains distinct-mask tables via
NextTableIndex. Verifier flips: dscp accepted (bound 0-63); multi-class accepted
when EVERY class carries a steering filter; prio + mark + unfiltered-multi-class
rejected. Backend refactored to per-interface aggregation; classify govppOps
wrappers moved to ops_linux.go (file-size split).

**Validation tiers** (A-6 apply-tier .ci still blocked on spec-finish-vpp-stub):
- Unit (fakeOps + verify + translate): all green -- dscp golden vectors, multi-
  class steering (1 table/2 sessions), mixed-field chaining (2 tables/family),
  reconcile drop-one-class, dscp + policer-name boundaries, prio/mark rejection.
- **Real-VPP evidence (authoritative):** `effective-vpp.py` extended with
  `run_traffic_dscp_evidence` + `run_traffic_multiclass_evidence`; both GREEN on
  VPP v25.10: dscp classify tables bound + steering + reconcile removal; two
  per-class policers bound via classify + reconcile removal. (The firewall
  evidence phase still fails on a separate pre-existing schema drift unrelated
  to this spec's traffic surface.)
- Verify-tier `.ci`: 020 repurposed to dscp out-of-range reject;
  020-vpp-accept-dscp-filter + 026-vpp-accept-multiclass (accept -> "vpp not
  connected"); 024-vpp-reject-prio + 025-vpp-reject-mark (rejection pins);
  011-vpp-reject-hfsc retained. NOTE: the traffic .ci functional suite is timing/
  stderr-capture sensitive and fails for CHANGED AND UNCHANGED tests alike on this
  heavily-loaded dev box (baseline 011/012/021 fail identically; expected strings
  appear in the aggregate daemon log). Per the ci-sleep rule no sleep baseline was
  raised; the pressure is reported. Logic confirmed by unit + real-VPP evidence and
  a direct `ze -` run showing the dscp-64 out-of-range rejection.
