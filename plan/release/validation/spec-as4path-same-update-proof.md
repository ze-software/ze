# Spec: as4path-same-update-proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | validation |
| Priority | P2 |
| Release reason | Inherited through-daemon AS4_PATH proof for shipped rewrite behavior remains owed. |
| Depends | - |
| Phase | - |
| Updated | 2026-10-22 |

## Task

Own row B's product evidence from
`plan/post-release/engineering/spec-ci-peer-harness-contract-backlog.md`:
observe AS_TRANS and the real ASN in the same UPDATE through the daemon.
The inherited RFC 6793 unit evidence does not replace socket proof.

## Constraints and acceptance

Read `docs/architecture/testing/ci-format.md` and assess the existing exact-frame
and ordered matching contracts first. Ordinary `hex`, `prefix` and `contains`
checks consume at most one rule per message; `consumeOrdered` can consume
several ordered needles, but those needles can span messages. Therefore two
successful checks alone do not prove that both attributes share one UPDATE.
The acceptance condition is through-daemon socket evidence tying both values
to one actual UPDATE, not two messages or merely unit-level encoding proof.
Record the concrete carrier and its discrimination before claiming this debt met.

A matcher enhancement is conditional on a demonstrated gap and remains owned
by the engineering parent; no speculative matcher change is commissioned.
There is no unconditional tooling dependency: use the existing contracts if
sufficient. If a gap is measured, record that prerequisite explicitly there.

## Provenance and scheduling

The original row came from retired `fixit-as4path-missing-on-rewrite`, via
`spec-fixit-redistribute-establishment-stall`, whose F1..F6 closure on
2026-08-23 did not discharge this separate proof. The parent retains the
complete original three-row history and durable-contract explanation.
This child owns only B's evidence; A/C remain historical, unconfirmed
observations in the parent, not commissioned product races or repairs.
Thomas has not scheduled this work. The inherited deferred/undesigned
state is preserved beneath the skeleton lifecycle; release classification
neither lifts that pause nor claims delivery or a new defect.
