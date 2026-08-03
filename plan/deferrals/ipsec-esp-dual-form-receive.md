# Deferrals -- spec-ipsec-esp-dual-form-receive

Source: `plan/spec-ipsec-esp-dual-form-receive.md`. Format: `ai/rules/planning.md`.

Created 2026-08-02. The spec's metadata table has named this shard since it was written, and
the file did not exist, so the spec's Goal Gate "Deferral shard resolved: no live row without
a destination" could not be evaluated at all. Nothing was lost: the rows below are the
residuals the spec's own "What is NOT done" section and the 2026-08-02 audit already record.
This file makes them enforceable.

Reference lists in this file are written as bullets, never as tables. Every pipe-delimited
line here is read by the deferral gate as a six-cell row.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-02 | spec-ipsec-esp-dual-form-receive | Write `test/ipsec-interop/scenarios/22-esp-encap-no-nat/`, the strongSwan proof that an encapsulated ESP form is accepted with no NAT present. The spec's Interop table names it and the directory does not exist | Gated by tier, not by difficulty. `test/ipsec-interop/` has no automated caller, so `scripts/dev/rfc_requirements.py` derives it as unrun and the gate refuses an RFC tag placed there. Writing the scenario is useful before that lands, but it earns no compliance evidence until the tree is wired, which is `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md` AC-1 to AC-3 territory for the ipsec tree | `plan/spec-ipsec-esp-dual-form-receive.md` | deferred |
| 2026-08-02 | spec-ipsec-esp-dual-form-receive | Write `test/ipsec-interop/scenarios/23-esp-form-change/`, the strongSwan-driven mid-SA ESP form change. Named by the spec's Interop table and by its "What is NOT done" section | Same tier gate as the row above. This scenario is ALSO the only peer-driven proof of AC-4, so it is not redundant with the `.ci`: the `.ci` proves Ze serves a form change, and the scenario proves a real peer's form change is served | `plan/spec-ipsec-esp-dual-form-receive.md` | deferred |
| 2026-08-02 | spec-ipsec-esp-dual-form-receive | Measure the throughput of the re-presented bare ESP form on a templated security association (risk R-1). The bare form is read off a raw socket and re-presented rather than taking the kernel fast path | Not a correctness gap and not a blocker for AC-4. The path is proven functional by `TestEncapOneStateAcceptsBothForms`. What is unmeasured is its cost, and no consumer of this spec depends on the number | `plan/spec-ipsec-esp-dual-form-receive.md` | deferred |

## Not deferred: the five residuals that stay in the spec

Listed rather than tabled, for the reason given above.

The 2026-08-02 audit found phase 5 open. Three of its items are deferred above because they
are gated on the interop tier or are a measurement nobody is waiting on. The rest stay in the
spec as live work, and are NOT deferrals:

- `test/ipsec/ipsec-esp-form-change.ci` -- the only proof of AC-4, and the only thing that
  makes the `docs/features/rfc-status.md` claim true. It runs in `ze-verify` on every push,
  so it is not gated by the interop tier and has no reason to be deferred.
- `test/ipsec/ipsec-esp-encap-no-nat.ci` -- user story 1, same reasoning.
- User story 3: `show vpn ipsec sa` exposes no ESP-form field. Add the field and its test, or
  strike the row from the spec. Either is a decision, not a deferral.
- `ai/RFC-REQUIREMENTS.md` regeneration, owed after the tagged-test rename.
- The spec's stale line anchors.

## The public claim currently runs ahead of the evidence

`docs/features/rfc-status.md` states that the platform limit is lifted and that a peer which
changes ESP form on an established security association is served. No test proves the mid-SA
change. This is not a deferral row, because the answer is not "later": either the `.ci` above
lands, or the sentence narrows to what is measured. It is recorded here so the choice is not
lost between sessions.
