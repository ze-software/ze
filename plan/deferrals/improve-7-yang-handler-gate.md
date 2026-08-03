# Deferrals: improve-7-yang-handler-gate

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-improve-7-yang-handler-gate (Known Limitations) | Strict unknown-key rejection at config verify (reject config keys absent from the schema); `validator.go` is permissive today | Opposite direction from improve-7's handler-completeness gate (config-not-in-schema vs schema-not-in-handler); recorded as follow-up candidate in the spec's Design Insights | `plan/spec-improve-7-yang-handler-gate-deferred-strict-unknown-key.md` | deferred |

