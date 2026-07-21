# Deferrals: iface-absent-link-graceful

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-iface-absent-link-graceful AC-3 | Full gokrazy L2TP appliance run proving graceful-skip end to end | Same two harness bugs stop web/l2tp from starting; the graceful-skip fix is proven on the real appliance boot ("interface config applied", no crash loop) | `plan/spec-finish-appliance-qemu-evidence.md` (work item added; re-homed 2026-07-16 -- the original destination was closed in `f42c2ccb2` with `plan/learned/1103`, whose :69-73 records that AC-3's end-to-end qemu run 'remains to be executed on a root host'. The two bugs that blocked it ARE fixed; only the run remains) | deferred |

