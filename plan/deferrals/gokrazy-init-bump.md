# Deferrals: gokrazy-init-bump

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-gokrazy-init-bump AC-6 | Full QEMU gokrazy L2TP appliance proof (build + boot + xl2tpd/pppd session) | Blocked by two appliance build/config-flow bugs unrelated to the bump (daemonRunning false-positive vs host sshd:22; ze init active-config shadows the build template); the bump itself is verified via the image build | `plan/spec-finish-appliance-qemu-evidence.md` (work item added; re-homed 2026-07-16 -- the original destination was closed in `f42c2ccb2` with `plan/learned/1103`, whose :69-73 records that AC-3's end-to-end qemu run 'remains to be executed on a root host'. The two bugs that blocked it ARE fixed; only the run remains) | deferred |

