# Known Failures

Pre-existing test failures tracked here per `ai/rules/git-safety.md` ("Before Any
Commit" → pre-existing failures >10 min): logged, not blocking unrelated commits.

## 2026-05-31 — dispatch single-marshal + stale plugin lists (15 packages)

**Status:** open. **Owner:** the in-flight dispatch/command specs
(`spec-command-strip-prefix`, `spec-command-verb-first`,
`spec-codec-callback-passthrough`, `spec-plugin-internal-keyword`).

**How it surfaced.** `make ze-verify` had been silently unrunnable on any dev host
that ran a QEMU or Docker test: those store a Go module cache under `tmp/`
(`tmp/qemu/gomodcache`, `tmp/linux-gomodcache`), and `go list ./...` walked into
it and failed with "directory ... outside main module or its selected
dependencies", aborting the gate before any test ran. A `tmp/go.mod` sentinel
(nested module → `go list` skips the subtree, recreated by
`scripts/evidence/qemu-run.py`) fixes that. With the gate runnable again, these
15 packages fail. None are caused by the runner / doctor / attrpool / l2tp /
QEMU-runner work committed alongside this entry — all those packages pass.

**Root causes (two classes):**

1. **`single-marshal OnExecuteCommand` (commit 30b025270).** Command handlers now
   return structured `any` (maps / `json.RawMessage`) and the SDK marshals once.
   Tests still do `assert.Contains(t, data, "substring")`, but testify on a
   `map`/`[]byte` matches keys/bytes, not substrings → false negatives even when
   the output is correct. Fix: assert on the marshaled JSON string (see the
   `rawString` helper added to `internal/test/plugins/fakeredist/fakeredist_test.go`
   as a sample — currently uncommitted, pending the coordinated fix).
2. **Stale plugin-registration expected lists.** New plugins (`flow-export`,
   `ldp`, `rsvp-te`, `bgp-filter-aspath-length`) are not in the hardcoded expected
   sets. Fix: add them.

**Failing packages / tests:**

| Package | Test(s) | Class |
|---------|---------|-------|
| `cmd/ze` | TestAvailablePlugins | 2 (stale list) |
| `internal/component/plugin/all` | TestAllPluginsRegistered | 2 (stale list) |
| `internal/component/bgp/plugins/adj_rib_in` | TestHandleCommand_Show, TestAdjRibInReplayArgsPassthrough, TestRevalidateInstalledRoute | 1 (marshal) |
| `internal/component/bgp/plugins/healthcheck` | TestShowEmptyProbes | 1 (marshal) |
| `internal/component/bgp/plugins/rs` | TestHandleCommand_Status | 1 (marshal) |
| `internal/test/plugins/fakeredist` | TestCommandEmitAdd, TestCommandEmitBurst | 1 (marshal) — sample fix staged uncommitted |
| `internal/plugins/fib/kernel` | TestFIBKernelShowInstalled | 1 (marshal) |
| `internal/plugins/fib/p4` | TestFIBP4ShowInstalled | 1 (marshal) |
| `cmd/ze/completion` | TestWordsShowProducesTabSeparatedOutput | 1 (marshal) |
| `cmd/ze/host` | TestSectionList | 1 (marshal) |
| `cmd/ze/yang` | TestDocCommandWithOutputParams | 1 (marshal) |
| `internal/component/cli` | TestCmdOptionChangesRedirectsToPipe | 1 (marshal) |
| `internal/component/config/yang` | TestBuildCommandTree, TestBuildCommandTreeCommandNodes | needs triage (also flagged pre-existing earlier) |
| `internal/component/cli/testing` | TestFunctionalETFiles | needs triage |
| `internal/exabgp/migration` | TestMigrateFileBasedTests | needs triage |

**Next step.** Class 1 and 2 are mostly mechanical but touch dispatch/command code
the in-flight specs are mid-refactor on; fixing them here would cross-commit. The
three "needs triage" rows may be real behavioral changes from the verb-first /
strip-prefix command work and need a closer look.
