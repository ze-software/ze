# Tests this commit weakens

Each row accepts one test weakening in the commit that carries this file. A
weakening removes an assertion, a case, or a test. `ai/rules/testing.md` forbids
every weakening. This file is the record of the ones a reviewer accepted.

Two gates read it. `c_test_weakening` (`.claude/hooks/pretool-writeedit.py`)
refuses the edit until this file names the test the edit weakens.
`scripts/dev/commit_helper.py` recomputes the weakenings of the paths the commit
names, and refuses a commit whose weakenings this file does not cover.

Both gates call one implementation, `scripts/dev/check_weakened_tests.py`, so
neither can disagree with the other about what a diff weakens or which row
covers it. `make ze-test-weakened-check` runs that checker over this file alone. It is
a stage of `ze-precommit-verify` in both modes, so a table this parser cannot read goes red
before any commit needs it.

A commit that weakens nothing carries the table with no rows.

## The commit carries this file

`scripts/dev/commit_helper.py` refuses a commit that weakens a test and does not
name `test/weakened.md` in its own `--file` list. A row that stays in the working
tree records nothing. The reason must sit in git history beside the weakening it
accepts, because that is the only place a later reader can find it.

## The file is replaced per commit

Delete the rows of the last commit. Write the rows of this one. This file never
accumulates.

The reason is the mechanism it replaces. A `test-relax:` comment stayed in the
test file forever, and a ceiling file capped their number. That corpus reached
601 tokens and 2,660 lines of prose across 413 test files. Nobody can read 601
justifications, so nobody read them, so writing one cost nothing.

A justification explains one diff. It is written at edit time and it is read at
review time. After the commit lands, it explains a change the reader of the test
file can no longer see. Storing that record permanently is what built the pile.

Git history holds every past entry. `git log -p -- test/weakened.md` shows the
rows of any commit beside the change they accepted.

## The test name

| Carrier | The name |
|---------|----------|
| Go, inside a top-level func | the enclosing `func TestXxx` |
| Go, outside every top-level func | the file stem |
| `.ci`, `.et` | the file stem, because each such file is one test |

`scripts/dev/rfc_tagged_scope.py` resolves each one. `go_func_units` returns the
top-level functions of a Go file with their names. `scope_reader` treats a file
that is not Go as one unit.

Row two is the case with no enclosing func to name. An `ignore` build tag in the
file header drops the whole file from the build, and a count can fall over
package-level code. The stem is then the only name available, so a weakening in
`a_test.go` is written as `a_test`.

A bare name is accepted when it resolves to exactly one weakened test in the
commit. Write `package.TestName` when it does not.
`TestNoGoFileBuildsMarkup` exists in `internal/component/lg` and in
`internal/component/web`, so a commit that weakens both writes
`lg.TestNoGoFileBuildsMarkup` and `web.TestNoGoFileBuildsMarkup`.

## The reason

Name what left the suite, and say why the commit is correct without it. A reason
that names no lost coverage tells the reviewer that the detector fired on a
change which removed none.

Every weakening kind needs a row at commit time, a falling count included. The
hook is narrower on purpose. An edit that only lowers a count gets a notice and
lands. Consolidating three cases into one table lowers a count exactly as
deleting a check does. So the commit gate asks for a row the hook did not, and
that row is where you say which of the two happened.

| Test | Reason |
|------|--------|
| TestReceivedUpdate_EBGPWireLazyASN4 | Its subject `ReceivedUpdate.EBGPWire` is deleted. The AS-path fold (`e2037e598`) moved eBGP prepending onto the edit-set rail, so the lazy generation this asserted no longer exists to assert. |
| TestReceivedUpdate_EBGPWireCachedASN4 | Its subject `ReceivedUpdate.EBGPWire` is deleted. Pointer equality on a second call describes a cache that is gone. |
| TestReceivedUpdate_EBGPWireLazyASN2 | Its subject `ReceivedUpdate.EBGPWire` is deleted. The two per-width slots it kept apart are gone with it. |
| TestReceivedUpdate_EBGPWireConcurrent | Its subject `ReceivedUpdate.EBGPWire` is deleted. There is no lazy initialization left to race. |
| TestReceivedUpdate_EBGPWireEvictionReturnsBuffers | Its subject is the two `ebgpWireSlot` handles released by `evictLocked` and `Delete`, both deleted. `TestForwardPoolBalanceLocalASOverride` and `TestForwardRSTranscodePoolBalance` keep covering the surviving `poolBuf` and `fwdHandles` releases on the same eviction path. |
| TestReceivedUpdate_EBGPWireErrorDoesNotPublish | Its subject is `errEbgpWireBufferExhaustedPoolAt` and the `EBGPWire` error path, both deleted. |
| extractFirstASN | Helper of the six tests above. Its only call site was inside `TestReceivedUpdate_EBGPWireLazyASN4`. The identically named helper in `internal/component/bgp/rib` is a different package and is untouched. |
| received_update_test | The count fell because the six `EBGPWire` tests and their helper left with the cache. `TestReceivedUpdateFields`, `TestReceivedUpdateWithdrawOnly`, `TestMsgIDAssignment`, `TestMsgIDMonotonic` and `TestReceivedUpdateAdoptedHandlesReturnedOnce` remain unchanged. |
| BenchmarkEBGPWireCacheHitParallel | Its subject `ReceivedUpdate.EBGPWire` is deleted. Its `perf.AllocCeilings` registration goes in the same change, which is what `make ze-alloc-check` checks. |
| BenchmarkEBGPWireCacheHitParallelMutexBaseline | The before-change comparator for the benchmark above. It takes `ebgpMu` and reads both slots directly, so it cannot outlive them. The measurement it reproduced is recorded in `docs/architecture/perf-round-3.md`. |
| ebgpWireMutexHit | The baseline benchmark's reproduction of the pre-lock-free hit path. It reads `ebgpMu` and the two slots, all deleted. |
