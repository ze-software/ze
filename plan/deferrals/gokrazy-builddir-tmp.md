# Deferrals: gokrazy-builddir-tmp

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-23 | spec-gokrazy-builddir-tmp | Unify how the two build paths SEED the appliance database: shell `ze init` + per-CERTNAME cert cache + `ZEFS=` on the make path, versus `assembleZeFS` from an appliance directory on the Go path. Also the make path's hardcoded `/perm` offsets, which the Go path discovers from the GPT | The D-1 audit found the two flows identical at the gok call and divergent only BELOW it, in seeding. Preparing the instance (this spec's goal) needed neither flow to change. Converging the seeding is separable, larger, and would have to preserve `ZEFS=` and the cert cache, which have no Go equivalent | plan/spec-gokrazy-builddir-tmp-deferred-build-flow-unification.md | deferred |
| 2026-07-23 | spec-gokrazy-builddir-tmp | Nothing gates the tracked builddir `go.sum` files against the root module, so they can drift without any check firing | Out of scope for relocating the build. Recorded as a Known Limitation in the source spec; it needs a home before that spec closes | plan/spec-gokrazy-builddir-tmp-deferred-build-flow-unification.md | deferred |
| 2026-07-23 | spec-gokrazy-builddir-tmp | The directory is still named `builddir` although the build no longer runs there | Mechanical rename, no behavior change; bundling it would have obscured the diff | plan/spec-gokrazy-builddir-tmp-deferred-build-flow-unification.md | deferred |
