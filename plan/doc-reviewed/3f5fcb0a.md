# Reviewed claims: session 3f5fcb0a

Each row records a docs claim a reader compared with a change that preserves
behavior. The rules and the check that reads this table are in
`docs/contributing/documentation-testing.md`.

| Doc | Source | Symbol | Commit | Reason |
|-----|--------|--------|--------|--------|
| `docs/architecture/system-architecture.md` | `internal/test/perfrunner/run.go` | `buildLinuxBinary` | `f171a5d47c` | the function only gained a context argument; it still cross-builds le through `linuxle.Argv` with the launcher's tags, so the page's claim about a CGO-free linux le is as true as before |
| `docs/architecture/system-architecture.md` | `internal/test/perfrunner/run.go` | `runPerf` | `f171a5d47c` | the sender now runs through `interoplab.Docker` instead of a raw `docker exec`, but le is still mounted read-only at `/usr/local/bin/le` (`containerLe`) and `le perf send` still measures the DUT, which is all the page claims |
| `docs/architecture/testing/ci-workflows.md` | `internal/le/interoplab/lab.go` | `Suite.Run` | `f171a5d47c` | only the call `releaseImage` became the exported `ReleaseImage`; the progress lines the page describes are unchanged |
| `docs/architecture/bgp/structural-forwarding.md` | `internal/component/bgp/reactor/adj_rib_out.go` | `adjOutPath` | `96a24194eb` | the struct only gained the `revocation` field, which ownership equality ignores; the claim is about the buffered frontier under `writeMu`, flush commit and sent-callback timing, none of which moved |
| `docs/architecture/bgp/structural-forwarding.md` | `internal/component/bgp/reactor/relay_recovery.go` | `recoverNLRIBatch` | `db457cb3c2` | the guard on a missing or non-negotiated family split into two guards with the same `continue`; no behavior changed |
| `docs/architecture/bgp/structural-forwarding.md` | `internal/component/bgp/reactor/relay_recovery.go` | `recoveryAdmission.admit` | `db457cb3c2` | the compound guard split in two and its body moved into `settleGone`; the same keys settle gone in the same cases |
| `docs/guide/route-reflection.md` | `internal/component/bgp/reactor/relay_recovery.go` | `recoverNLRIBatch` | `db457cb3c2` | the guard on a missing or non-negotiated family split into two guards with the same `continue`; no behavior changed |
| `docs/architecture/core-design.md` | `internal/component/bgp/reactor/forward_pool.go` | `fwdBatchHandler` | `9aafe62f7f` | the early returns for a destination with no Established session or no connection now settle recovery admissions; they return before the write deadline is set, and the deadline is still set before the batch and cleared after it, as the page says |
| `docs/architecture/forward-congestion-pool.md` | `internal/component/bgp/reactor/forward_pool.go` | `fwdBatchHandler` | `9aafe62f7f` | the change settles recovery admissions on the destination-closed early returns; the per-item source check (Established, not stopping, same generation) and the teardown check after each batch are unchanged |
