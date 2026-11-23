# Handover 62: land the test-deletion hook fix, then lint the site and commit packages

Written 2026-11-23 by the session that ran the 9 November update report, at its
context threshold. Thomas ordered both fixes ("fix"). No spec is claimed; this is
an owner-ordered fix, done directly.

## 1. The test-deletion hook false refusal (written, uncommitted, not run)

**Defect.** The PreToolUse Bash guard `bashTestDeletion`
(`internal/le/hookruntime/bash.go`) refused a `./le commit create ... file
<path>_test.go ...; ...; rm -f "$S" "$M"` line as a test deletion. Its word list
(`deletionWords`/`readDeletionWords`, `testdeletion.go`) took every word of the
line once any word was `rm`, `*/rm` or `-delete`, so every test path anywhere on
the line was judged deleted.

**Fix on disk.** The guard reads each simple command on its own
(`deletionReach`/`readDeletionReach`, `lineDeletes` in `testdeletion.go`;
`stagesFeed` in `parsedcommands.go`). A test path counts only when a deleting
command reaches it: as its own word (substitutions included), through a name the
line assigns or loops over, from an earlier pipeline stage, from its own
here-document or here-string, or as a relative operand joined to a `cd`/`pushd`
operand. A line handed to a shell (`bash -c`, `eval`, a shell reading its script
from a here-document) is read as its own line, at most 8 levels. An unparsable
line keeps the old text judgment. `plainRmArguments` and the line-wide list are
deleted.

**Files.** `internal/le/hookruntime/{testdeletion.go,bash.go,parsedcommands.go,
bash_test.go,readerline_test.go,verdicts_test.go,shellparse_test.go,
testdata/guard-verdicts-head.jsonl}`, `internal/le/ai/hooks/fixtures.go`.

**Test changes that need the independent judge (`ze-test-judge`) before landing:**
- seven expectations flipped from refused to allowed, each a line where the test
  path is only an argument of another command (`rm -f "$f" && grep x
  internal/a_test.go`, `rm -f $(cat list) && grep x internal/a_test.go`, `rm -f x
  && cat a > internal/a_test.go`, `( rm -f scratch ); grep x internal/a_test.go`,
  `rm -f $f && grep x internal/a_test.go`, `cd x && rm -f scratch && grep x
  internal/a_test.go`, `cmp --frobnicate tmp/x internal/a_test.go; rm tmp/x`),
  each with a new sibling that really deletes the test and must still be refused;
- `bashTestDeletion` removed by hand from 14 rows of
  `testdata/guard-verdicts-head.jsonl`;
- `bashTestDeletion` moved from the "reader guards" list to the "never lifts" list
  in `TestReaderLiftableChecksAreTheReaderGuards`.
New: `TestTestDeletionJudgesEachCommandByWhatItDeletes` (the exact refused commit
line must pass; the same line with an `rm` of a `_test.go` must be refused; 14
other deleting shapes refused).

**Owed, in order (main thread):**
1. `tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"; ./le job run label unit-pkg quiet command go test -race -tags "$tags" -run 'TestGuardVerdictsOfHeadStand' ./internal/le/hookruntime/ -args -update`
   (appends rows for the new corpus lines; refused rows must list `bashTestDeletion`, allowed ones must not).
2. `./le test unit pkg path ./internal/le/hookruntime/ run 'TestTestDeletion|TestReaderLiftableChecksAreTheReaderGuards|TestGuardVerdictsOfHeadStand|TestLinesBashReadsOtherwiseAreNotRead|FuzzShellParse'`
3. `./le test unit pkg path ./internal/le/hookruntime/`
4. `./le test unit pkg path ./internal/le/ai/hooks/ run 'TestEveryStoredDigestMatchesTheTreeItRatchets'`: expect red with the new `hookSourcesDigest` literal; paste it into `internal/le/ai/hooks/fixtures.go`, rerun green.
5. `./le ai hooks unit`
6. `./le go lint run scope "./internal/le/hookruntime/ ./internal/le/ai/hooks/"`
7. Independent `ze-test-judge` agent on the test changes above; record its verdicts through the approval route and carry one `Weakening-approved:` line per test in the commit.
8. Commit through `./le commit create`, one command per step (create, run the script, clean up separately).

Once green, five open rows of `plan/journal/hook-existing-patterns-false-positive.md`
become fixed (2026-11-11 `git update-index`; 2026-11-12 x3: the reader `for` loop,
the `mv` out of `test/draft`, the "ze data rm" here-document; 2026-11-14
here-document note), owned by `plan/post-release/engineering/spec-hook-le-data-arguments.md`.
Resolve them per `plan/journal/README.md`. The 2026-11-14 `testPath`-under-`tmp/`
row is a different defect; leave it.

## 2. Lint owed for fixes committed today

`./le go lint run scope "./internal/le/site/ ./internal/le/commit/"` after
9e09521758 (site record step records the union of dirty paths) and 8bb4694e2f
(runChangeChecks unparam). Their tests passed; lint was not rerun.

## 3. Owner decisions still open

| Decision | Evidence |
|---|---|
| Push `main` | `main` is about 75 commits ahead of origin; the published site (gh-pages 6b9783d0bc) links commits not yet on GitHub |
| Containers in the next update report and the milestones | Shipped in the appliance build (`ze_container`), declaration Scope still `future`, specs under `plan/post-release/` |
| Correct the 9 November report's "every RFC Ze implements" for the 5,188 figure | That count covers every reviewed requirement list, including RFCs Ze does not implement; the homepage's 4,7xx figure is the implemented-RFC one |

The correction of the 9 November report's graceful-restart line is already
recorded in `website/changes/fragments/bgp-graceful-restart-rfc-defects.md`.
