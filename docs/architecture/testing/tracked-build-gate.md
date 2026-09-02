# The Tracked-Build Gate

`./le repository tracked-build check` compiles what git holds. Every other build and
test target in this repository compiles the WORKING TREE, so a commit that
lands a consumer while its producer stays uncommitted passes every gate and
breaks HEAD for everyone else.

<!-- source: internal/le/repository/trackedbuild/actions.go -- Answer -->

On 2026-08-04 four commits broke the retired tracked-build path in one day.
The current replacement, `./le repository tracked-build check`, compiles the
committed population and catches that class.

## Structural type checking and final linking

<!-- source: internal/le/staticcheckfeaturematrix/actions.go -- Answer -->

`./le staticcheck-feature-matrix check` type-checks working-tree production
and `_test.go` sources. It derives N+2 rows from the N unique manifest features:
distro all-on, bare core, and one row for each omitted feature.
The matrix covers those direct omissions. It makes no guarantee for arbitrary
combinations with multiple omitted features.

Those N+2 rows are what the target judges when it is typed on its own. Inside a
verify run it judges only the rows the change set can move, keeping the distro
all-on and bare-core rows always: `docs/architecture/testing/verify-freshness-scope.md`.

### One matrix, six pieces

A verify run does not judge those rows in one stage. `check part <index> of
<count>` judges the rows dealt to one piece, and the stage population runs six
pieces (`staticcheckParts`, `internal/le/verify/engine/stages.go`). CI deals the
stage list to its shards round robin, so the six pieces run on six different
shards. Every piece runs on every verify, so the whole matrix is still judged.

The deal is round robin over the DERIVED rows (`Matrix.Part`), never an
assignment written by hand, so a feature gate added to `feature-gates.txt` lands
in one piece for free. Each piece names the rows it judged in its own log, which
is the only log a reader of one shard has. Typing
`./le staticcheck-feature-matrix check` with no part judges every row, as before.

One Staticcheck run is bounded at 90 seconds for each row it judges
(`deadlinePerRow`, `internal/le/staticcheckfeaturematrix/judge.go`), so a 7-row
piece gets 10m30s and an undivided 38-row run gets 57m. The flat 25 minutes this
replaces bounded the whole matrix: CI run 33450825487 measured 23m36s inside it
and most other runs exceeded it, which reported a slow gate as an unjudgeable
one. `ZE_STATICCHECK_DEADLINE` still names an absolute bound for a run of any
size.

Staticcheck stops after package and test-variant type checking.
`./le repository tracked-build check` supplies committed-tree final-link proof for
its six tracked configurations, not for every shipped build flavor. Keep both
stages live because they judge different populations and compiler boundaries.

## Extraction

The tree is extracted with `git archive`, not `git worktree add --detach`.
`git archive` writes nothing into `.git/`, so a killed run leaves no worktree
registration to prune in a checkout that several sessions share.

`-trimpath` is load-bearing, not hygiene. Without it the compile action ID
carries the absolute source directory, so every fresh scratch path is a full
rebuild (measured 36s, then 36s again). With it the shared build cache is
reused across scratch paths: 36s once, then 5.7s.

`vendor/` must be included. The commit package's index extraction intentionally
omits it, so that helper cannot serve this gate: a tree without `vendor/` does
not build.

The parent must drain `git archive`'s stdout pipe. If `tar` dies early, the
still-open read end gives the writer no EPIPE and `git archive` blocks forever
on a full pipe.

## Flavors and why each names a file

Six build-tag flavors are compiled, at about 45s warm against a 25-minute
`./le verify current mode full`. `ze_chaos`, `ze_perf`, `ze_analyze` and `ze_core ze_ssh` are
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
`ai/rules/precommit-verify.md` requires the check right after a commit script that
carried Go, and the gate is in `STRUCTURAL_GATES`, so the next commit is
refused while HEAD is red.

Running it during `./le commit create` against a prospective tree was rejected.
One generated script can hold several commit blocks, and an overlay of a later
block's paths onto HEAD omits an earlier block's producer. That produces a false
red and makes the gate untrustworthy.

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
- Adding a binary flavor outside `internal/le/repository/trackedbuild/matrix.go` does not
  extend the gate. `TestEveryFlavorNamesATagGatedAnchorFile` requires each
  shipped row to name the tag-gated file it exists to compile.
- `REV=<commit-ish> ./le repository tracked-build check` judges a past commit,
  so a break found later remains bisectable.

## Two shell and tooling traps

`git cat-file -e <sha>:<absent path>` exits 128, the same code it uses for a
corrupt repository. Treating 128 as a real failure and 1 as "absent" breaks
every green case. `git ls-tree --name-only` exits 0 and prints nothing for an
absent path, which is the distinction the guard needs.

`go run <prog>` collapses every nonzero exit of the program into its own exit
1, so a test cannot tell "the commit does not build" from "the gate could not
judge". The tests compile the checker and execute the binary.
