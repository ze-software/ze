| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-15 | spec-plugin-reload-applies-or-refuses | `TestUnionTimeout` (`pkg/plugin/sdk/union_test.go`) | `go test -race` failed once with "race detected": the test writes `u.sweepInterval` after `NewUnion` started the sweep goroutine that reads it; a rerun passed | open: set the interval before the goroutine starts, as a `NewUnion` parameter or option |
