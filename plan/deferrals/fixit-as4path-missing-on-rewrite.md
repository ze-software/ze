# Deferrals: fixit-as4path-missing-on-rewrite

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-fixit-as4path-missing-on-rewrite (functional test) | The `.ci` proving AS4_PATH on the wire cannot be written: `internal/test/peer/checker.go` consumes ONE RULE PER MESSAGE, and a meaningful AS4_PATH assertion needs TWO facts about a single UPDATE (AS_TRANS in the 2-octet AS_PATH, AND the real >65535 ASN in AS4_PATH). Either alone cannot distinguish the fix from the bug: AS_TRANS alone is what the BUG produces. The same limit is why `test/plugin/bgp-rs-asn4-transcode.ci` cannot pass today with its three `contains` rules on one UPDATE (proven: 1 rule + EOR-first goes green, 3 goes red). Multi-rule-per-message matching would unblock both | The RFC 6793 fix is unit-tested at the `wireu` seam with derived wire bytes and mutation-verified, so the behaviour IS pinned; what is missing is the through-the-daemon proof `ai/rules/testing.md` wants. Changing the matcher is test-harness work in `internal/test/`, which was settled and committed today in `aaefef8ce` and is a distinct change with its own risk | `plan/future/spec-ci-peer-harness-contract-backlog.md` (re-homed 2026-08-23: F1-F3 landed and `spec-fixit-redistribute-establishment-stall` closed, so the `.ci` harness contract now lives in `docs/architecture/testing/ci-format.md` and this row needs a spec that can own the matcher change) | deferred |

