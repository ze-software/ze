# 1342 -- Tracked Build Gate

## Context

On 2026-08-04 four commits broke `make ze` at HEAD in one day (7abe8a07e,
025a74b72, aa1b7a4d4, fa372140b). Each was the same defect: a consumer was
committed while the producer of the symbol it reads stayed uncommitted in the
working tree. Every gate in this repository was green at the moment each commit
was made, and Thomas found the first one by running `make ze` and getting a
compile error. The cause is structural, not carelessness: `make ze`, `ze-verify`,
`ze-lint-changed`, `ze-rfc-check` and every test target build the WORKING TREE, so
they all see the uncommitted producer. Nothing in the pipeline COMPILED what git
actually held, so nothing could see this class at all.

## Decisions

- Extract with `git archive` over `git worktree add --detach`: `git archive`
  writes nothing into `.git/`, so a killed run leaves no worktree registration
  needing `git worktree prune` in a checkout several sessions share.
- `-trimpath` on every build, and it is load-bearing rather than hygiene: without
  it the compile action id carries the absolute source directory, so a fresh
  scratch path is a full rebuild every time (measured 36s, then 36s again). With
  it the shared GOCACHE is reused across scratch paths: 36s once, then 5.7s.
- Build `./...` per flavor over building each flavor's own main package:
  measured +1.8s, and it type-checks every package the tag set selects, not only
  the ones a binary imports.
- Six flavors over the Makefile's full matrix: about 45s warm against a
  25-minute `ze-verify`. `ze_chaos`, `ze_perf`, `ze_analyze` and `ze_core ze_ssh`
  were dropped as developer tools or near-duplicates.
- Each flavor names the tag-gated FILES its own tags select, over naming only an
  anchor package and over trusting `go build`'s exit code. Both weaker forms were
  tried first and both were inert: see the first two Gotchas.
- Wired as a `ze-verify` stage in both modes AND added to `STRUCTURAL_GATES` in
  `scripts/dev/commit_helper.py`, over a verify stage alone: a red HEAD build is
  deterministic, so it must not be parkable in `plan/known-failures/` or waved
  through with `--unverified`.
- A dedicated `--broken-head-fix "<reason>"` escape, over reusing the owner-only
  `--structural-red-ok` or leaving no escape at all. See the deadlock in
  Gotchas: this gate's red is the one that a commit CLEARS rather than precedes.
- Rejected running the gate inside `commit_helper.py create` against the
  prospective commit tree, which would have been the only PREVENTIVE placement.
  `create` is called once per commit BLOCK, and `--append` builds several blocks
  into one script, so an overlay of block 2's paths onto HEAD omits block 1's
  producer and reports a false red. A commit gate that cries wolf gets bypassed.

## Consequences

- The gate is DETECTIVE, not preventive: it judges what git already holds, so the
  commit that breaks HEAD is caught by the next verify, not by itself. What makes
  that bite is `ai/rules/git-safety.md` step 7, which now requires
  `make ze-tracked-build-check` right after running a commit script that carried
  Go, plus the structural-gate refusal that blocks the NEXT commit.
- `REV=<commit-ish>` makes any past commit judgeable, so a break found later is
  bisectable without hand-rolling the extraction.
- `_test.go` files are outside the gate forever: `go build` never compiles them.
  A test file committed without its fixture producer stays invisible here.
- Adding a binary flavor to the Makefile does not automatically extend the gate.
  `TestTrackedBuildPrimaryFlavorMatchesMakeZe` only pins the daemon flavor
  against the Makefile's `ze` target; the other five are a judged list. Every row
  IS checked for an anchor, and `TestTrackedBuildOSGatedFlavorsPinGOOS` fails any
  row whose anchor package is linux-gated while the row pins no GOOS.

## Gotchas

- **Adding this gate to `STRUCTURAL_GATES` created a deadlock, and the gate's own
  purpose is what created it.** Every other structural gate reads the working
  tree, so its red is fixed before the next commit. This one reads HEAD, so its
  red is fixed BY the next commit, the one landing the missing producer. The
  refusal blocked exactly that commit, leaving the owner-only
  `--structural-red-ok` as the only route and HEAD broken for everybody until the
  owner was available. A gate whose remedy its own refusal forbids is worse than
  no gate. Round five of the review found it; four earlier rounds did not.
- **A package resolves if ANY ONE of its files survives the constraints.**
  `cmd/ze/main.go` carries no build constraint, so `go list ./cmd/ze` succeeds
  under `-tags ze_bogus`, and an anchor-PACKAGE check was inert for five of the
  six flavors: a mistyped or dropped tag left them green while compiling none of
  the dispatch code the tags exist to select. The package-count floor could not
  cover it either, since the bogus tag set still selected 636 of 637 packages.
  Only naming a tag-gated FILE ties the answer back to the tags.
- **`git cat-file -e <sha>:<absent path>` exits 128, not 1** -- the same code it
  uses for a corrupt repository. Treating 128 as "real failure" and 1 as "absent"
  therefore breaks every green case. `git ls-tree --name-only` exits 0 and prints
  nothing for an absent path, which is the distinction the guard needs.
- **`go build ./...` EXITS 0 over a pattern that matched nothing buildable**, so
  a green exit code is not evidence that anything compiled. `cmd/ze-installer` is
  `//go:build linux && ze_installer`, so the installer flavor compiled zero
  packages on a macOS host and the gate printed "OK (every flavor of the
  committed tree compiles)". A gate written to close a fail-open had one of its
  own. The fix is arithmetic rather than trust: each flavor names an anchor
  package that `go list` must resolve, and the run counts the packages selected
  against a floor.
- `go build -o <dir> ./...` REFUSES a pattern that matches no main package
  ("go: no main packages to build"), so the first version failed on a
  library-only tree for a reason unrelated to what was committed. Dropping `-o`
  entirely is correct: with a wildcard, `go build` compiles and discards.
- `go run <prog>` collapses every nonzero exit of the program into its own exit
  1, so the test could not tell "the commit does not build" (1) from "the gate
  could not judge" (2). The tests compile the checker first and exec the binary.
- **A new rule step can contradict a paragraph three screens above it in the same
  file.** Step 7 of the commit workflow now requires `make ze-tracked-build-check`
  after the commit script, while "Explicit commit requests are a fast path" in the
  same file forbids rerunning gates at commit time. Independent review caught it;
  the author had read both paragraphs and seen no conflict. When a rule file is
  long, adding a step means re-reading the file for what now disagrees with it.
- `commit_helper.py`'s existing `extract_head_into` excludes `vendor/` on purpose
  (the discovery-index generators skip it). It cannot be reused here: this module
  is vendored, so a tree without `vendor/` does not build at all.
- The parent's read end of `git archive`'s stdout pipe stays open after `tar`
  exits, so a `tar` that dies early gives the writer no EPIPE and `git archive`
  blocks forever on a full pipe. The pipe must be drained explicitly.
- 7abe8a07e and 025a74b72 both fail on the same two references, in
  `cmd/ze/hub/main_reload.go` and `cmd/ze/hub/service_web.go`. The
  web-certificate consumer was committed several commits BEFORE its producer, so
  one orphan reference sat broken across three commits while each commit message
  read as a fix.

## Files

- `scripts/checks/tracked_build.go` (new): the gate
- `scripts/checks/tracked_build_test.go` (new): entry-point tests, including the
  red-when-producer-uncommitted mutation proof
- `Makefile`: `ze-tracked-build-check` target, `help-dev` entry
- `scripts/status/verify_run.go`, `scripts/status/verify_run_test.go`: the stage
  in both modes and both goldens
- `scripts/dev/commit_helper.py`: `STRUCTURAL_GATES`
- `ai/rules/git-safety.md`: "Your Working Tree Is Not What You Committed", plus
  the fast-path carve-out that would otherwise forbid step 7
- `ai/rules/CORE.md`: the regenerated always-on digest of git-safety
- `ai/rules/repo-maintenance.md`, `ai/INDEX.md`, `docs/contributing/testing.md`
