# 1097 -- followup-vpp-traffic (dscp police-by-dscp + multi-class steering)

## Context

The VPP traffic backend (after 1096) accepted `filter protocol` via the classify +
policer-classify pipeline but rejected `filter dscp`, `filter mark`, `qdisc prio`, and
every multi-class config at config-verify (exact-or-reject). Two questions were left to
the user because the spec was internally inconsistent about them, and both were answered
2026-07-10:

1. **`match dscp` under vpp = POLICE-BY-DSCP** (not the spec's original QoS
   record/map/mark remark). A real-VPP spike had proved record/map/mark only REMARKS
   egress DSCP; it cannot police DSCP-matched traffic. The user chose the parity reading:
   classify the DSCP/TOS bits and steer to the class policer, the SAME pipeline as
   `filter protocol`.
2. **`qdisc prio` STAYS REJECTED** -- VPP has no priority scheduler; mapping prio to a
   DSCP egress-map would be a silent semantic substitution exact-or-reject forbids. AC-4
   resolves as rejection-retained with an evidence-backed error; `egressMapFromPrioClasses`
   was never built.

This work landed phase 3 (dscp) + phase 6 (multi-class steering), closing the spec.

## Decisions

- **DSCP is a sibling of protocol, not a remark.** `dscpClassifyVectors` matches the
  DiffServ Code Point at absolute frame offsets: IPv4 TOS byte 15 mask 0xFC, match
  `dscp<<2`; IPv6 traffic-class straddles bytes 14/15 (not byte-aligned in the first IPv6
  word) mask 0x0F/0xC0, match `(dscp>>2)&0x0F` / `(dscp&3)<<6`. Chosen over
  QosRecord/EgressMap/Mark because that pipeline remarks but cannot police by match.
- **Multi-class steering groups by MASK, not by class.** Per interface, per family, all
  filtered classes' steerings are grouped by field mask: ONE table per distinct mask with
  a session per class (each session's `HitNextIndex` = its class policer). Same-field
  multi-class (two classes on different protocols) is one table + N sessions -- no
  chaining. Chaining (`ClassifyAddDelTable.NextTableIndex`) is reserved for mixed-field
  (a protocol table chained to a dscp table). This is fewer tables than the prior
  session's "one table per class chained" framing.
- **Multi-class requires EVERY class to carry a steering filter.** A single class may be
  unfiltered (egress policer-output = interface rate limit). But two unfiltered classes
  stack in series (min(rates)), and mixing an unfiltered egress class with filtered
  ingress classes crosses feature directions. Reject the unfiltered-in-multi-class case.
- **Reject duplicate steering (same type+value) across classes.** Classify sessions are
  keyed by match value; two classes selecting the identical protocol/dscp would collide
  and VPP keeps only the last (silent last-wins). `verifyNoDuplicateSteering` rejects it.
- **Prio + mark stay rejected** with dedicated, actionable, evidence-backed errors
  (`errQdiscPrioNotSupportedByBackend`, `errFilterMarkNotSupportedByBackend`).

## Consequences

- The classify data model changed from per-class single binding to per-interface,
  per-family table CHAINS with a name->index policer map (`classifyBinding{ip4Tables,
  ip6Tables, policers}`). Reconcile deletes every table in the chain and every filtered
  policer the new state no longer keeps; the output-policer reconcile and the classify
  reconcile guard each other so a policer migrating output<->classify is never
  double-deleted.
- `backend_linux.go` crossed the 600-line soft cap after the refactor; the govppOps
  production adapter (policer + classify wrappers) moved to `ops_linux.go`.
- `policerName`/`policerNamePrefix` and `classSteers` moved to the cross-platform
  `translate.go` so the (build-tag-free) verifier and the linux backend share one
  definition. `policerName` uses `textbuf` (no-sprintf-alloc hook enforces this even on
  the cold verify path).
- Real-VPP evidence is the authoritative apply-tier validation (A-6: the stub cannot run
  a full traffic Apply). `effective-vpp.py` gained `run_traffic_dscp_evidence` +
  `run_traffic_multiclass_evidence`, both green on VPP v25.10.

## Gotchas

- **`list match { key "type" }`** in the YANG means a class can hold at most ONE match per
  type (one protocol, one dscp, one mark). So "two protocol matches in one class" is
  impossible; the only multi-mask case is protocol+dscp in one class/interface. And
  `list class { key "name" }` makes duplicate class names unreachable -- no code guard
  needed for either.
- **DSCP is not byte-aligned in IPv6.** The 8-bit traffic-class field spans byte 14's low
  nibble (TC[7:4]) and byte 15's top two bits (TC[3:2]); a single-byte mask is wrong. The
  two-byte mask 0x0F/0xC0 was confirmed accepted by real VPP.
- **`NextTableIndex` is honored and read-back-visible.** `show classify tables` shows
  `NextTbl = successor index` on a chained head. A miss on the head falls through. (The
  spike's first run mis-parsed the table index and set next-table to a garbage value 510,
  which VPP still stored -- proving the field is honored regardless.)
- **Multi-session per table works with distinct `HitNextIndex`.** `show classify table
  index N verbose` shows `elts 2` with `next_index 40` / `41` -- N classes steer to N
  policers from one table.
- **The traffic `.ci` functional suite is timing/stderr-capture sensitive.** On a heavily
  loaded box (concurrent sessions + Docker) the daemon startup + 5s WaitConnected exceeds
  the `wait.py` window, so the SIGTERM cancels in-flight ops before the natural
  verify-reject / "vpp not connected" line lands -- and CHANGED AND UNCHANGED tests
  (011-hfsc, 012-not-connected, 021-tc-backend) fail identically. The expected strings
  appear in the aggregate daemon log. Do NOT raise the sleep baseline (ci-sleep rule);
  rely on unit + real-VPP evidence, report the pressure. A direct `ze -` run confirmed the
  dscp-64 out-of-range rejection fires.
- **Parse-level vs backend-verifier rejection.** `dscp value 64 out of range` is emitted
  by the config/parse layer (before the backend verifier). Prio/mark rejection comes from
  the backend verifier (`RunVerifier` in `parseAndVerifyTrafficSections`, OnConfigVerify).

## Files

- `internal/plugins/traffic/vpp/translate.go` -- `dscpClassifyVectors`,
  `filterClassifyVectors`, `classSteers`, `policerName`/`policerNamePrefix` (moved here,
  textbuf), dscp offset constants
- `internal/plugins/traffic/vpp/classify_linux.go` -- per-interface classify aggregation:
  `collectClassSteerings`, `applyInterfaceClassify`, `buildFamilyChain`,
  `groupSteeringsByMask`, chain-aware reconcile + policer teardown
- `internal/plugins/traffic/vpp/ops.go` -- `classifyAddDelTable` gains `nextTableIdx`
- `internal/plugins/traffic/vpp/ops_linux.go` -- NEW: govppOps production adapter moved
  out of backend_linux.go (policer + classify wrappers; ClassifyAddDelTable.NextTableIndex)
- `internal/plugins/traffic/vpp/backend_linux.go` -- applyInterface refactored to collect
  steerings + build once; multi-class allowed; cross-map reconcile guard
- `internal/plugins/traffic/vpp/verify.go` -- dscp accepted (0-63); multi-class accepted
  iff every class filtered; prio + mark + duplicate-steering rejected; AC-8 comments
  retargeted to this summary
- `internal/plugins/traffic/vpp/{translate,verify,apply}_test.go` -- dscp golden vectors,
  multi-class steering + chaining, reconcile, boundaries, rejection pins
- `test/traffic/020-vpp-reject-dscp-filter.ci` (repurposed: dscp out-of-range),
  `020-vpp-accept-dscp-filter.ci`, `024-vpp-reject-prio.ci`, `025-vpp-reject-mark.ci`,
  `026-vpp-accept-multiclass.ci`
- `scripts/evidence/effective-vpp.py` -- dscp + multi-class evidence phases
- `docs/features.md` -- VPP Traffic Control Backend row (police-by-dscp, multi-class)
- `plan/learned/1097-followup-vpp-traffic.md` -- decisions, assumptions, review gate, closure
