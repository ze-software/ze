| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-15 | spec-plugin-reload-applies-or-refuses | `TestUnionTimeout` (`pkg/plugin/sdk/union_test.go`) | `go test -race` failed once with "race detected": the test writes `u.sweepInterval` after `NewUnion` started the sweep goroutine that reads it; a rerun passed | fixed: 9549c1b106 passes the sweep interval to `newUnion` before the worker starts, and the test no longer writes the field (`TestUnionConfiguredBeforeStart`) |
