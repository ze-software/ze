# Env key read by production code and registered only by a test

`env.Get` is FATAL on a key nothing registered, which is the right answer: a
typo in a key name must not read as "unset". The trap is a production path that
reads a key whose only `env.MustRegister` lives in a `_test.go` file of some
OTHER package. The daemon registers it through a composition root the test
binary does not link, so the code works in the product and kills any test binary
whose package happens to reach that path. The failure names the key, not the
missing registration, so the reader looks for a typo.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-07 | daemon-backed-command-catalog | `internal/core/resolve/resolve.go`, which reads `ze.storage.blob`, reached by `TestShowCommandPayloadsAreStructured` (`internal/component/plugin/server/response_format_audit_test.go`) | `go test ./internal/component/plugin/server/` under the full feature tags dies with `FATAL: env.Get called with unregistered key: ze.storage.blob` after 468 tests pass. The package FAILS with no `--- FAIL` line, because the process exits inside a running test. Every file involved is clean at HEAD: `internal/component/plugin/server/`, `internal/core/resolve/` and `internal/component/resolve/`. The only `env.MustRegister` calls for that key are in `internal/component/doctor/doctor_test.go`, `internal/plugins/crashes/readiness_test.go` and `internal/component/support/crashes_module_test.go`, all test files of other packages. Walked into while running the touched packages for this spec | not fixed, and not this spec's surface |
