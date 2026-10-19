# Reviewed claims: session 3f5fcb0a

Each row records a docs claim a reader compared with a change that preserves
behavior. The rules and the check that reads this table are in
`docs/contributing/documentation-testing.md`.

| Doc | Source | Symbol | Commit | Reason |
|-----|--------|--------|--------|--------|
| `docs/architecture/system-architecture.md` | `internal/test/perfrunner/run.go` | `buildLinuxBinary` | `f171a5d47c` | the function only gained a context argument; it still cross-builds le through `linuxle.Argv` with the launcher's tags, so the page's claim about a CGO-free linux le is as true as before |
| `docs/architecture/system-architecture.md` | `internal/test/perfrunner/run.go` | `runPerf` | `f171a5d47c` | the sender now runs through `interoplab.Docker` instead of a raw `docker exec`, but le is still mounted read-only at `/usr/local/bin/le` (`containerLe`) and `le perf send` still measures the DUT, which is all the page claims |
| `docs/architecture/testing/ci-workflows.md` | `internal/le/interoplab/lab.go` | `Suite.Run` | `f171a5d47c` | only the call `releaseImage` became the exported `ReleaseImage`; the progress lines the page describes are unchanged |
