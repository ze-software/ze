# Reviewed claims: session e23fbcef

Each row records a documentation claim compared with a change that preserves
the behavior the claim states. The ledger contract is in
`docs/contributing/documentation-testing.md`.

| Doc | Source | Symbol | Commit | Reason |
|-----|--------|--------|--------|--------|
| `docs/contributing/terminal-demos.md` | `internal/test/runner/server_process.go` | `ServerProcess.Stop` | `12afa215f6` | Stop delegates to the shared stop helper with wholeGroup false. The daemon still receives leader TERM, the caller's ten-second grace, owned group cleanup and a sole-reaper join before store removal. Cleanup errors remain joined with the browser failure. |
