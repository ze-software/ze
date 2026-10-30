# A test polls producer state without the producer's lock

The test reads a field the code under test writes from its own goroutine, with
no synchronization, so `-race` fails the package whether or not the behaviour
is right.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | route-policy-0-umbrella (found, not caused) | rpki `TestCacheGroupResyncsAfterSingleCacheRestart` (`rtr_cache_restart_test.go`) against `cacheGroup.load` (`rtr_session.go`) | `go test -race` on internal/component/bgp/plugins/rpki at 87482aab03 reports a data race: the test reads cache-group state inside `Eventually` while `cacheGroup.load` writes it | open: read the state through the cache group's own locked accessor, or expose one, then rerun under -race |
