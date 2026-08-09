# The Tracked-Build Gate

`make ze-tracked-build-check` compiles what git holds. Every other build and
test target in this repository compiles the WORKING TREE, so a commit that
lands a consumer while its producer stays uncommitted passes every gate and
breaks HEAD for everyone else.

<!-- source: scripts/checks/tracked_build.go -- extraction, flavors, anchors, package floor -->

On 2026-08-04 four commits broke `make ze` in one day with that same defect.
Every gate was green at each commit. Nothing in the pipeline compiled the
committed population, so nothing could see the class at all.

## Extraction

The tree is extracted with `git archive`, not `git worktree add --detach`.
`git archive` writes nothing into `.git/`, so a killed run leaves no worktree
registration to prune in a checkout that several sessions share.

`-trimpath` is load-bearing, not hygiene. Without it the compile action ID
carries the absolute source directory, so every fresh scratch path is a full
rebuild (measured 36s, then 36s again). With it the shared build cache is
reused across scratch paths: 36s once, then 5.7s.

`vendor/` must be included. `commit_helper.py`'s existing `extract_head_into`
excludes it on purpose for the index generators, so it cannot be reused here: a
tree with no `vendor/` does not build at all.

The parent must drain `git archive`'s stdout pipe. If `tar` dies early, the
still-open read end gives the writer no EPIPE and `git archive` blocks forever
on a full pipe.

## Flavors and why each names a file

Six build-tag flavors are compiled, at about 45s warm against a 25-minute
`ze-verify`. `ze_chaos`, `ze_perf`, `ze_analyze` and `ze_core ze_ssh` are
dropped as developer tools or near-duplicates. Each flavor builds `./...`
rather than its own main package: it costs 1.8s more and type-checks every
package the tag set selects, not only the ones a binary imports.

Each flavor names a tag-gated FILE its own tags select. Two weaker forms were
tried first and both were inert:

- **A package anchor.** `cmd/ze/main.go` carries no build constraint, so
  `go list ./cmd/ze` succeeds even under a bogus tag. The anchor-package check
  was inert for five of the six flavors, and the package-count floor could not
  cover it either, because a bogus tag set still selected 636 of 637 packages.
- **The exit code alone.** `go build ./...` EXITS 0 over a pattern that matched
  nothing buildable. `cmd/ze-installer` is `//go:build linux && ze_installer`,
  so the installer flavor compiled zero packages on a macOS host and the gate
  printed a pass. The fix is arithmetic: each flavor names an anchor package
  that `go list` must resolve, and the run counts the selected packages against
  a floor.

`go build -o <dir> ./...` refuses a pattern that matches no main package, so
`-o` is dropped entirely. With a wildcard, `go build` compiles and discards.

## Detective, not preventive

The gate judges what git already holds, so the commit that breaks HEAD is
caught by the next run, not by itself. Two things make that bite:
`ai/rules/git-safety.md` requires the check right after a commit script that
carried Go, and the gate is in `STRUCTURAL_GATES`, so the next commit is
refused while HEAD is red.

Running it inside `commit_helper.py create` against the prospective tree was
rejected. `create` is called once per commit BLOCK and `--append` builds several
blocks into one script, so an overlay of block 2's paths onto HEAD omits block
1's producer and reports a false red. A commit gate that cries wolf gets
bypassed.

### The deadlock the structural listing created

Every other structural gate reads the working tree, so its red is cleared
before the next commit. This one reads HEAD, so its red is cleared BY the next
commit, the one landing the missing producer. The refusal blocked exactly that
commit. The escape is a dedicated `--broken-head-fix "<reason>"`, not the
owner-only `--structural-red-ok`, because otherwise HEAD stays broken for
everybody until the owner is available. Round five of the review found this;
four earlier rounds did not.

## Limits

- `_test.go` files are outside the gate forever, because `go build` never
  compiles them. A test file committed without its fixture producer stays
  invisible here.
- Adding a binary flavor to the Makefile does not extend the gate.
  `TestTrackedBuildPrimaryFlavorMatchesMakeZe` pins the daemon flavor against
  the Makefile `ze` target; the other five are a judged list. Every row is
  checked for an anchor, and `TestTrackedBuildOSGatedFlavorsPinGOOS` fails a
  row whose anchor package is Linux-gated while the row pins no `GOOS`.
- `REV=<commit-ish>` makes any past commit judgeable, so a break found later is
  bisectable with no hand-rolled extraction.

## Two shell and tooling traps

`git cat-file -e <sha>:<absent path>` exits 128, the same code it uses for a
corrupt repository. Treating 128 as a real failure and 1 as "absent" breaks
every green case. `git ls-tree --name-only` exits 0 and prints nothing for an
absent path, which is the distinction the guard needs.

`go run <prog>` collapses every nonzero exit of the program into its own exit
1, so a test cannot tell "the commit does not build" from "the gate could not
judge". The tests compile the checker and execute the binary.
