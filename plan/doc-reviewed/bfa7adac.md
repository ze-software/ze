# Reviewed claims: session bfa7adac

Each row records a docs claim a reader compared with a change that preserves
behavior. The rules and the check that reads this table are in
`docs/contributing/documentation-testing.md`.

| Doc | Source | Symbol | Commit | Reason |
|-----|--------|--------|--------|--------|
| `docs/architecture/testing/runner-architecture.md` | `internal/le/test/deployment/daemonbuild.go` | `runDaemonBuild` | `3894cc7f89` | only the GOCACHE fallback changed, from the bootstrap tmp/go-cache to the checkout's cache/go-cache; the build still runs under the verb's signal root and stops with it, which is what the page states |
