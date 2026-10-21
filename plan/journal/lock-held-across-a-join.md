| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-13 | - (rs-replacement design review r10) | `Reactor.cleanup` (`bgp/reactor/reactor.go`) with `fwdPool.Stop` (`forward_pool.go`) | `cleanup` holds `r.mu` while `fwdPool.Stop` waits for every forward worker; a worker that takes `r.mu` (`markSessionUp`, `emitCongestionEvent` from `onResumed`) never exits, so stop wedges. Seen live 2026-10-21 | fixed: the pool stops before `cleanup` takes `r.mu`; `TestReactorStopsWithACongestedForwardWorker` is red without it |
