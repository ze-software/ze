# VPP Traffic Backend: Police by DSCP and Multi-Class Steering

The [VPP traffic backend](fw-7-traffic-vpp.md) accepted `filter protocol`
through the classify plus policer-classify pipeline, and rejected `filter dscp`,
`filter mark`, `qdisc prio` and every multi-class config at config-verify. This
adds DSCP policing and multi-class steering.

Two questions the owner settled:

1. **`match dscp` under the vpp backend means POLICE BY DSCP.** A real-VPP spike
   proved that the QoS record, map and mark pipeline only REMARKS egress DSCP and
   cannot police DSCP-matched traffic. The chosen reading is parity with
   `filter protocol`: classify the DSCP bits and steer to the class policer.
2. **`qdisc prio` STAYS REJECTED.** VPP has no priority scheduler. Mapping prio to
   a DSCP egress map is a silent semantic substitution, which exact-or-reject
   forbids. `egressMapFromPrioClasses` was never built.

## DSCP is a sibling of protocol

<!-- source: internal/plugins/traffic/vpp/translate.go -- dscpClassifyVectors, filterClassifyVectors, classSteers -->

`dscpClassifyVectors` matches the DiffServ Code Point at absolute frame offsets:

- IPv4: TOS at byte 15, mask `0xFC`, match `dscp << 2`.
- IPv6: the traffic class straddles bytes 14 and 15, so the mask is `0x0F` on
  byte 14 and `0xC0` on byte 15, matching `(dscp >> 2) & 0x0F` and
  `(dscp & 3) << 6`.

**DSCP is not byte-aligned in IPv6.** The 8-bit traffic-class field spans byte
14's low nibble (TC[7:4]) and byte 15's top two bits (TC[3:2]). A single-byte
mask is wrong. Real VPP accepts the two-byte mask.

## Steering groups by mask, not by class

<!-- source: internal/plugins/traffic/vpp/classify_linux.go -- collectClassSteerings, buildFamilyChain, groupSteeringsByMask -->

Per interface and per family, every filtered class's steerings are grouped by
FIELD MASK: one classify table per distinct mask, with one session per class, and
each session's `HitNextIndex` is that class's policer.

Two classes on different protocols are therefore one table with two sessions, and
no chaining. Chaining through `ClassifyAddDelTable.NextTableIndex` is reserved
for the mixed-field case, a protocol table chained to a DSCP table. This is fewer
tables than "one table per class, chained".

Read-back evidence from real VPP: `show classify tables` reports
`NextTbl = successor index` on a chained head, and a miss on the head falls
through. `show classify table index N verbose` reports `elts 2` with
`next_index 40` and `41`, which is two classes steering to two policers from one
table.

## Rejections that remain

<!-- source: internal/plugins/traffic/vpp/verify.go -- multi-class rules, verifyNoDuplicateSteering -->

- **Multi-class requires EVERY class to carry a steering filter.** A single class
  may be unfiltered, which is the egress policer-output interface rate limit. Two
  unfiltered classes stack in series at `min(rates)`, and mixing an unfiltered
  egress class with filtered ingress classes crosses feature directions.
- **Duplicate steering (the same type and value) across classes is rejected.**
  Classify sessions are keyed by match value, so two classes selecting the same
  protocol or DSCP collide and VPP keeps only the last one, silently.
- **`qdisc prio` and `filter mark` are rejected** with dedicated errors,
  `errQdiscPrioNotSupportedByBackend` and `errFilterMarkNotSupportedByBackend`.

## What the YANG keys already guarantee

`list match { key "type" }` means a class holds at most one match per type: one
protocol, one dscp, one mark. Two protocol matches in one class are impossible,
so the only multi-mask case is protocol plus dscp in one class. `list class
{ key "name" }` makes a duplicate class name unreachable. Neither needs a code
guard.

## Reconcile

<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- applyInterface, cross-map reconcile guard -->
<!-- source: internal/plugins/traffic/vpp/ops.go -- classifyAddDelTable, nextTableIdx -->

The classify data model is per-interface, per-family table CHAINS with a
name-to-index policer map. Reconcile deletes every table in the chain and every
filtered policer the new state no longer keeps. A migration from output to
classify keeps the same policer object but explicitly removes its old output
attachment: VPP 26.06 updates preserve attachments. Failure to unbind aborts the
apply and triggers backend recovery of the prior configuration and tracked
bindings. A later failure likewise restores an already removed output binding.
Recovery errors are returned, not hidden. The two reconcile paths retain
policers still referenced by the other path rather than deleting migrated objects.

`policerName`, `policerNamePrefix` and `classSteers` live in the
build-tag-free `translate.go`, so the verifier and the Linux backend share one
definition. `policerName` uses `textbuf`, which the no-sprintf-alloc hook
enforces even on the cold verify path.

After a Ze restart, policer discovery returns names but no indices. Startup
reconciliation therefore recreates named Ze policers before building new
classify chains. Tracked same-process policers use `PolicerUpdate`, with a
name check at the cached index first. See [startup reconciliation](fw-7-traffic-vpp.md#a-ze-restart-with-external-vpp)
for the replacement interval and exclusive-interface ownership assumption.
<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- cleanupStartupOrphans, applyInterface -->

Anonymous tables from a previous Ze process are not swept. A desired classify
configuration replaces its interface's old chain heads, but startup cannot
recover the old table ownership from policer names. If the restarted config
removes classification entirely, this path does not identify or unbind those
old classify chains. This is an existing limitation, not a cleanup guarantee.
<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- cleanupStartupOrphans -->

## Where a rejection comes from

`dscp value 64 out of range` is emitted by the config parse layer, before the
backend verifier runs. The prio and mark rejections come from the backend
verifier, through `RunVerifier` in `parseAndVerifyTrafficSections` under
`OnConfigVerify`. Knowing which layer answers tells you which test can see it.

## Evidence

<!-- source: internal/le/test/deployment/actions.go -- Answer -->

Real-VPP evidence is the authoritative apply-tier validation, because the stub
cannot run a full traffic Apply. The earlier apply evidence was recorded on VPP
v25.10. It does not establish same-config Ze restart reconciliation on VPP 26.06:
the corrected daemon-restart proof exposed duplicate policer creation there.
The source repair requires a fresh full native deployment run before a new
restart claim.
<!-- source: internal/le/test/deployment/vppevidencerun.go -- runTrafficInterface, runTrafficClassify -->
