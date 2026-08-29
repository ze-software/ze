# Deferrals: fixit-static-interface-nexthops

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-static-interface-nexthops functional-proof | test/static/005+006 .ci are needs-linux QEMU, deferred to CI (AC-7) | live-server/QEMU constraint, deferred to CI | plan/future/spec-finish-appliance-qemu-evidence.md | done |



Closed 2026-08-29 after verifying the producer rather than the row: the static fsuite is wired at `internal/le/qemu/alltests.go` and run nightly by `.github/workflows/qemu-nightly.yml`; the needs-linux tests are `test/static/static-interface-nexthop-no-backend.ci` and `static-table-interface.ci`.
