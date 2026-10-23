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
| 2026-09-22 | - | SDK identity bootstrap and `newObserver` (`internal/test/fixture/fixture.go`) | External observers aborted on `ze.plugin.name` because only the plugin CLI registered it. | fixed: Move the declaration beside the SDK transport keys. The isolated composition smoke fails before the move and passes after it. |
| 2026-09-07 | mgmt-version-header-suppress | `internal/component/config/apply_env.go`, `envPlumbingTable`, the table that carries an `environment` YANG leaf into an env key | A plumbing row that spells its key as a literal registers nothing, so `env.Set` ends the daemon the first time an operator writes that leaf. The config package cannot register the key either, because the reader lives in another package and only that package's link decides whether the registration is present | fixed: the key is an exported constant declared beside its reader, with `env.MustRegister` in the same file (`internal/core/version/version.go`, `EnvKeyHideVersion`), and the plumbing row names the constant. The import that carries the spelling then carries the registration into every binary that links the row |
| 2026-09-07 | daemon-backed-command-catalog | `internal/core/resolve/resolve.go`, which reads `ze.storage.blob`, reached by `TestShowCommandPayloadsAreStructured` (`internal/component/plugin/server/response_format_audit_test.go`) | `go test ./internal/component/plugin/server/` died with `FATAL: env.Get called with unregistered key: ze.storage.blob`, a FAIL with no `--- FAIL` line | fixed: obsolete. No Go file at HEAD reads or registers `ze.storage.blob`; `internal/core/resolve/resolve.go` resolves storage from `ze.config.dir` through `StoreDir` |
