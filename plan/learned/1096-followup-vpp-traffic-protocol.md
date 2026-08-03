# 1096 -- followup-vpp-traffic-protocol

## Context

The VPP traffic backend rejected every `filter` type at config-verify (exact-or-reject),
so `traffic { control { backend vpp } }` could only do interface-wide rate limits (one
policer bound to egress via `PolicerOutput`). The goal was to wire `filter protocol <n>`
so only matching traffic is policed, reusing the "proven" firewall classify pipeline. A
real-VPP spike (Docker `ligato/vpp-base`, VPP v25.10) drove the design because several of
the spec's assumptions (steering mechanism, IPv6 next-header, offsets) were unvalidated and
the firewall precedent turned out NOT to be packet-validated.

## Decisions

- Steer via `ClassifyAddDelSession.HitNextIndex = policer index` with `action=0` -- chosen
  over `SET_METADATA`/`OpaqueIndex` because it is byte-for-byte what VPP's own CLI
  `classify session ... policer-hit-next <name>` produces (verified by reading the session
  back on real VPP). The firewall's `SET_METADATA` Limit path is unproven on real traffic.
- Table shape: `skip_n_vectors=0` with a full-width 2-vector (32-byte) absolute-offset mask
  (IPv4 protocol at frame byte 23, IPv6 next-header at byte 20) -- chosen over VPP's CLI
  `skip=1` + 16-byte match because the binary `classify_add_del_session` validates the match
  length against skip+match vectors and rejects a short match with `INVALID_VALUE(-7)`.
- Filtered class uses the ingress `policer-classify` feature (`PolicerClassifySetInterface`
  binding per-family ip4/ip6 tables), NOT the egress `PolicerOutput` path; the filtered
  policer is tracked in the classify binding, not `interfaceOutputPolicers`, so reconcile
  never spuriously unbinds an output feature that was never applied.
- One `filter protocol` matches its protocol in BOTH families (IPv4 + IPv6), matching the
  netlink backend's parity (each `filter protocol` emits a v4 and v6 selector).

## Consequences

- The classify offsets/steering are now ground-truthed against real VPP and pinned by golden
  vectors (`protocolClassifyVectors`) + an evidence assertion that the table is ATTACHED
  (the R-1 "table created but never attached" killer). This is the foundation the remaining
  phases (dscp 3-step, prio egress-map, mark, multi-class steering) build on.
- VPP classify tables are anonymous (no Ze-owned name), so startup orphan cleanup CANNOT
  reclaim them by name like policers. In-process reconcile deletes them by tracked index; a
  table left by a dead process is inert (unbound) memory reclaimed only by a VPP restart.
- `spec-finish-vpp-stub.md` must add `sw_interface_dump` + `policer_add_del` handlers before
  any apply-tier `.ci` traffic test can run against the stub (A-6 is broken -- only
  `classify_add_del_table` was added here). Until then, validation is unit (fakeOps) +
  real-VPP evidence.

## Gotchas

- `INVALID_VALUE(-7)` on `ClassifyAddDelSession` is NOT about `HitNextIndex` -- it is the
  match-length-vs-skip mismatch. Two hours were lost blaming the steering field. The fix was
  the table's `skip`/mask width, not the session.
- Go's zero value for `HitNextIndex` is `0`, NOT the binapi `default=4294967295`. The binapi
  `default=` tags are for codegen/JSON, not Go struct init -- a struct built without setting
  a field gets Go's zero, which VPP may reject.
- `effective-vpp.py` was doubly broken by earlier renames and blocked ALL VPP evidence:
  `ensure_linux_binaries` built the nonexistent `./cmd/ze-test` (ze-test is `-tags ze_test`
  on `./cmd/ze`), and the traffic configs used the retired top-level keyword
  `traffic-control {` instead of nested `traffic { control { ... } }`. Both fixed here.
- Packet-level policing could not be injected on a VPP loopback: pg frames punt because
  `ip4-unicast` is not enabled without an IP, and even with one loopback pg does not traverse
  the classify feature. State-level programming (session byte-matches VPP's canonical
  `policer-hit-next`) is the contract; packet-level proof needs a non-loopback interface.
- The firewall classify Limit path (the spec's "proven prior art") is NOT packet-validated
  and uses `SET_METADATA` steering that this work did not adopt -- do not assume it polices.

## Files

- `internal/plugins/traffic/vpp/translate.go` -- `protocolClassifyVectors` + offset constants
- `internal/plugins/traffic/vpp/ops.go` -- classify* methods on the vppOps seam
- `internal/plugins/traffic/vpp/backend_linux.go` -- govppOps classify wrappers + apply branch
- `internal/plugins/traffic/vpp/classify_linux.go` -- NEW: create/undo/reconcile pipeline
- `internal/plugins/traffic/vpp/verify.go` -- protocol accepted (bound-checked), dscp/mark rejected
- `internal/plugins/traffic/vpp/binapi_imports.go` -- classify anchor re-added
- `internal/plugins/traffic/vpp/{translate,apply,verify}_test.go` -- golden/apply/verify tests
- `test/scripts/vpp_stub.py` -- `classify_add_del_table` handler (returns NewTableIndex)
- `scripts/evidence/effective-vpp.py` -- protocol evidence phase + 2 pre-existing bug fixes
- `docs/features.md` -- VPP Traffic Control Backend row updated
- `plan/learned/1097-followup-vpp-traffic.md` -- assumption verdicts + progress log
