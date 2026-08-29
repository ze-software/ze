# Deferrals: improve-7-yang-handler-gate

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-improve-7-yang-handler-gate (Known Limitations) | Strict unknown-key rejection at config verify (reject config keys absent from the schema); `validator.go` is permissive today | Opposite direction from improve-7's handler-completeness gate (config-not-in-schema vs schema-not-in-handler); recorded as follow-up candidate in the spec's Design Insights. Destination corrected 2026-08-29 at improve-7's closure: the row named `spec-improve-7-yang-handler-gate-deferred-strict-unknown-key`, a spec that was never written, so nothing on disk could ever satisfy it. Homed at the improve umbrella, which carries it as child 9 | `plan/spec-improve-0-umbrella.md` | deferred |

