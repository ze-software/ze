# Current fragment test draft

This is the complete reviewed source packet, retained for the owner's token
pause. It is not compiled or executed and supplies no native acceptance.

Read `PRODUCERS-r2.md` first. It supersedes the original `HANDOFF.md` claims
about address-only readiness and queue allocation proving payload insertion.
Both `.go.txt` files and `r2-graft.patch` are required: the five-root original,
the R2 readiness/FIFO helper, then the exact graft onto the original.

First pass `TestNativeIPv6CarrierOriginalTraffic` on the current matching
kernel/image. Then promote the two source files using the mappings in
`PRODUCERS-r2.md`, apply the graft and compile/run all five roots in Main.
Preserve current production and fixture changes; do not restore old snapshots.

Remove this draft packet through `le commit remove` in the same commit that
lands its promoted integration tests. The owning progress and remaining work
are in `plan/post-release/features/spec-pppoe-vpp-dataplane.md`.
