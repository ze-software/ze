# Suite shares one persistent store

Every functional test in a session resolves the same on-disk store, so a test
that writes state into it changes what every LATER test in the session reads.
The defect presents as an order-dependent failure: the test passes alone, passes
first, and fails after the writer has run. Nothing in the failing test names the
writer, so the reader looks for a code regression in the wrong place.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-20 | - | functional runner | `setupBinShims` (`internal/test/runner/runner.go`) symlinks `ze` into the session tree so `DefaultConfigDir` resolves `<session>/etc/ze`, and every test in the session therefore opens the SAME `database.zefs`. A test that stores a local auth credential leaves `meta/auth/local/username=admin` in it, and `liveLocalUsers` (`cmd/ze/hub/main.go`) then reports one boot user for every later daemon. `main.go` prefers per-user auth over `ze.api-server.token`, so four `test/reload/mgmt-guard-*` tests that configure a boot token, or no auth at all, met an authenticated REST listener they never asked for. Two of them (`mgmt-guard-reload-auth-rebuild`, `mgmt-guard-reload-unbuilt-transport`) fail only with the polluted store and pass with a fresh one; the other two failed for an unrelated reason. Confirmed by moving the store aside and re-running the four | not fixed: recorded. The fix is a per-test config dir, or a runner step that reseeds the store between tests, and either one is a change to the runner's process model rather than to a test |
