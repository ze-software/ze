# A read lock taken twice on one path

A Go `sync.RWMutex` is not reentrant, and the failure is not the second
acquisition on its own. It is the writer that arrives between the two. A waiting
writer blocks every new reader, so the second `RLock` waits for the writer, the
writer waits for the first `RLock` to be given back, and the first one is held by
the goroutine now waiting. Nothing times out and nothing logs.

The shape is an accessor that locks, called from inside a callback that the same
type invokes with that lock already held. Each half reads correctly on its own,
so the defect is invisible at both sites and appears only in a stack dump of a
process that has stopped.

The tell is a type whose iteration takes its own lock and whose small getters
take it too. Read what the walk needs BEFORE the walk starts, and say on the
getter that it must not be called from inside one.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-22 | record-answers-3-zero-alloc | bgp rib show pipelines | `inboundSource.Next`, `protocolInboundSource.Next` (`internal/component/bgp/plugins/rib/rib_pipeline.go`) and `newBestSource`'s collect (`rib_pipeline_best.go`) each called `peerRIB.IsAddPath(fam)` from inside a `peerRIB.IterateSorted` callback. `IterateSorted` runs the callback under `PeerRIB.mu.RLock` and `IsAddPath` takes that same lock, so a `PeerRIB.Remove` arriving between them wedges all three goroutines for good. It is reachable in production and not only under a test: `handleReceivedStructured` (`rib_structured.go`) gives `peerMu` back before phase 2 removes anything, so any withdrawal on a peer whose RIB a show command is walking can hit it, and the RIB plugin then answers nothing ever again. Found by a new test that walks the best-path table beside a goroutine removing the routes it reads, which hung for 3 minutes until the package timeout dumped the stack: goroutine 97 in `sync.RWMutex.Lock` inside `(*PeerRIB).Remove`. Five other `IsAddPath` call sites in the same package already read the flag before iterating, so the correct shape was beside the defect the whole time | fixed at all three sites. `PeerRIB.AddPathFamilies` returns the per-family flags as a copy, taken once before the walk starts, and `IsAddPath` now states that it MUST NOT be called from inside an iteration callback. `TestBestPipelineWalkSurvivesConcurrentUpdates` is the regression, and it runs the walk under its own deadline so the failure reports the wedge rather than waiting out a suite timeout |
