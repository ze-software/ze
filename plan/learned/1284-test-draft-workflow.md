# test-draft-workflow

A functional test under development now lives in `test/draft/<suite>/`, where no
suite and no repo-wide gate can see it, and is promoted with a plain `mv` when it
is green.

## The problem it solves

`test/<suite>/` is live. Every `make ze-verify` in the checkout runs it, and this
repo routinely has several sessions working the same tree at once. A test being
written or changed therefore reddened verify for people who had nothing to do with
it, and they had to spend the diagnosis to discover the red was not theirs.

There was no way to iterate on a `.ci` without that cost.

## Why it needed more than a directory

Suite discovery is a NON-recursive glob (`internal/test/runner/record_parse.go`
`Discover`: `filepath.Glob(dir + "/*.ci")`), so a subdirectory was already
invisible to the runner. That part was free.

The part that was not: SIX gates walk `test/` recursively, and each would have
seen a draft.

| Gate | Producer |
|------|----------|
| accept-only lint | `internal/test/runner/accept_only.go` |
| BGP frame-length fixtures | `internal/test/runner/ci_fixture_test.go` |
| ci-sleep ratchet | `scripts/dev/verify_wiring_docs.py` |
| observer-recover check | `scripts/dev/ci_observer_recover_check.py` |
| dispatch-command check | `scripts/checks/ci_dispatch_commands.go` |
| inert-test check | `scripts/checks/inert_tests.go` |

Each now prunes the directory. `TestDraftDirIsInvisibleToRepoGates`
(`internal/test/runner/draft_dir_test.go`) asserts the skip is still spelled in
each producer, so a scanner that drops it fails a test instead of surfacing as a
mystery red in a stranger's session. A NEW repo-wide `.ci` scanner must skip
`test/draft/` and add a row there.

## Why gitignored

The exclusions above are local. Ignoring the tree makes the guarantee independent
of all six of them: CI checks out git, so an ignored directory does not exist
there at all. `!test/draft/README.md` keeps the convention discoverable from a
fresh clone, and `TestDraftDirIsGitignored` pins both halves.

## The mechanism

`SuiteDir(baseDir, suite, draft)` (`internal/test/runner/draft_dir.go`) is the one
place the discovery root is chosen; `--draft` is wired into every suite entry
point (`ci_runner.go`, `cmd_bgp.go`, `cmd_editor.go`, `cmd_web.go`, `cmd_vpp.go`).
A suite that built the path by hand and forgot the draft branch would silently run
the REAL tests under `--draft`, which is the one outcome the whole thing exists to
prevent, so there is no second copy of that conditional.

```
$EDITOR test/draft/plugin/my-test.ci
ze-test bgp plugin --draft -a
python3 scripts/dev/stress-repro.py "bgp plugin --draft" --test 1 --any-failure
mv test/draft/plugin/my-test.ci test/plugin/
ze-test bgp plugin -a
```

Promotion is a plain move because the draft is untracked: the destination just
appears as a new file. No `git mv`, no staging.

## Proving an exclusion, rather than asserting it

The mechanism was verified with a deliberately-broken draft carrying an
unjustified `time.sleep(3.0)`, a bogus frame length and no assertions. Every gate
stayed green and none mentioned it; the sleep ratchet read exactly
`80 <= ceiling 80`, which it could not have if the draft's sleep had been counted.
Worth repeating whenever a new gate is added: a skip that is merely believed is not
a skip.

## First real use, same session

`test/plugin/eor.ci`. Drafting let the diagnosis add three debug log knobs without
touching the live test, and the capture killed the working hypothesis in 6 ms of
log: both End-of-RIB markers were already on the wire from `sendInitialRoutes`, and
the ze-peer had closed the session before the observer's (unreachable) sends ran.
See `plan/learned/1283-fixit-ci-plugin-suite-nine.md` Shape 4.

Without the incubator, that instrumentation would have had to go into the live
test, where every concurrent verify in the tree would have picked it up.

## Related

- `ai/skills/ze-test.md` -- the skill that makes this the standing procedure
- `ai/rules/testing.md` -- the blocking directive
- `test/draft/README.md` -- the contract, including the gate table
- `docs/functional-tests.md` -- "Writing a Test: Draft First"
