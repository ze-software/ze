# Door: development tooling and tests

<!-- door-base: internal/le internal/test cmd/ze/ze_le_register.go test -->

Read `ai/doors/system.md` first. This page covers `le`, the development
launcher, and the test trees it runs. Here "outside" does not mean a customer. It means:

- every other agent session that works in this checkout at the same time
- the owner's Linux and macOS machines
- CI

## Where it sits

- **`le` is a personality of `cmd/ze`.** Both use the command registry and
  renderers in `internal/component/command`. There is no second command engine
  (`docs/architecture/core-design.md`, "Command engine and composition roots").
- **The rule is directional.** A shipped `ze` never links an `internal/le`
  package. `TestNormalZeLinksNoInternalLe` (`cmd/ze/ze_le_personality_test.go`)
  measures it over every shipped flavor. Only the `ze_le` tag
  (`cmd/ze/ze_le_register.go`) links le into `ze`, and no shipped build sets it.
- **`le` is allowed to link the product root `internal/component/plugin/all`**, because a
  command inventory or a grammar check answers what `ze` registers. It serves
  only `le <tool>` paths, so a product root such as `interface` is unknown to
  `le` (`leroot.Owns`, `internal/le/le/root/leroot.go`).
- **Areas register.** A tool is one package under `internal/le/` that calls
  `leroot.Register` and `leroot.RegisterActions` from `init()`, plus one blank
  import in `internal/le/register.go`.
- **The hooks are le.** Every hook in `.claude/settings.json` runs
  `$CLAUDE_PROJECT_DIR/le ai hooks <name>`. The code is
  `internal/le/hookruntime` (`.claude/hooks/README.md` names each family's owner).
- **Tests.** Functional `.ci` files live under `test/<suite>/` and run through
  `./le test <suite>` on the functional runner (`internal/test/runner`,
  `docs/architecture/testing/runner-architecture.md`). `.et` editor files run
  through `./le test editor`. Interop scenarios live under `test/interop*/` and
  run through `internal/le/interoplab` (`docs/architecture/testing/interop.md`).

## Who depends on it from outside

| Outside party | What it relies on | What breaks |
|---------------|-------------------|-------------|
| Every concurrent agent session | The hooks run on each Bash, Write and Edit call | A hook that panics or refuses wrongly blocks every session at once (`plan/learned/HOOK-FRICTION.md`) |
| Every session, through `./le` and its installed copy `bin/le` | The dispatcher runs the newest build in `bin/builds/<UTC commit time>-<sha>/le`, built from a `git archive` export of HEAD, keyed by the git tree hash of le's inputs, smoke-tested in a dot-prefixed staging directory and renamed into place, so no session's uncommitted edit reaches the shared le and a running le keeps its inode. A build that does not start is moved aside to `bin/builds/.failed-<name>` (`./le rollback` does the same by hand) | A build that writes in place, or a build from the working tree, breaks the le another session runs (`docs/contributing/running-commands.md`, "Launcher builds do not overwrite running binaries") |
| Every session, through the shared index | `./le commit create` stages only the named files | A loose `git add` carries another session's work (`ai/rules/git-safety.md`) |
| Sessions sharing one machine | `./le job run` admission, `./le scratch`, the shared Docker daemon | Oversubscription, lost scratch, removed containers (`running-commands.md`, `ai/META-DECISIONS.md`) |
| The owner's macOS and Linux machines, and a QEMU guest mounting the checkout | A machine whose platform differs from the newest build's header builds and runs `bin/builds/<time>-<sha>-<GOOS>-<GOARCH>/le`. The shipped tree type-checks on darwin | A gate whose verdict depends on the host (`plan/journal/gate-verdict-depends-on-the-machine.md`, one of the largest journal classes) |
| CI (`.github/workflows/`) | `./le test qemu`, `./le test integration`, `./le build host-driver`, `./le setup docker kernel` | A renamed action breaks a workflow nobody runs locally |
| A Go version bump | le activates the patched GOROOT before dispatch, and a refusal leaves hooks and the commit flow working | All sessions lose their hooks if dispatch stops (`docs/contributing/patching-the-standard-library.md`) |

## Boundaries

- **The package sits at the path its command name predicts**:
  `le spec journal` is `internal/le/spec/journal`.
  `TestEveryCommandIsFoundAtThePathItsNamePredicts` (`internal/le/group_test.go`)
  refuses an exception. A space is a directory level, a hyphen joins one name
  (`docs/architecture/cli/command-namespacing.md`).
- **An action stays callable as Go and answers through the shared renderer**,
  never a private JSON mode (core-design, "Command engine").
- **A before-dispatch hook never prints, never waits, returns nothing**
  (`leroot.RegisterBeforeDispatch`, `internal/le/le/root/beforedispatch.go`).
- **Never import `internal/le` from product code**, not even for a helper.
  `TestNormalZeLinksNoInternalLe` goes red. A helper both sides need lives
  outside `internal/le`.
- **A bare `go test` omits the feature tags**, so plugins do not register and
  unrelated tests fail. Run one package through
  `./le job run label unit-pkg command go test -tags ...`
  (`running-commands.md`, "A bare `go test` is not `./le test unit`" and
  "Testing one package while you develop it").
- **A `.ci` that needs a Linux kernel feature carries `option=needs-linux`** and
  runs in the QEMU VM (`ai/rules/platform-linux.md`).
- **A compiled Go fixture returns an error when an assertion fails**
  (`ai/patterns/functional-test.md`, `ai/rules/testing.md`).

## Read before code

| Read | Question it answers |
|------|---------------------|
| `docs/architecture/core-design.md`, "Command engine and composition roots" | How le and ze share one codebase, and how a tool registers |
| `docs/contributing/running-commands.md` | Launcher, tags, job admission, scratch, session identity, full disk |
| `docs/contributing/testing.md`, `docs/functional-tests.md` | Which suite runs what, and how to run one |
| `docs/architecture/testing/ci-format.md`, `runner-architecture.md` | The `.ci` format and the runner that reads it |
| `docs/architecture/testing/interop.md` | Scenario discovery, the peer per protocol, vacuity traps |
| `docs/guide/chaos-testing.md`, `docs/performance.md` | `./le chaos run` and `./le perf` |
| `plan/learned/HOOK-FRICTION.md` | Known false positives of each hook, before you change one |
| `ai/patterns/functional-test.md` | The shape of a new `.ci` test |
| `ai/rules/testing.md`, `ai/rules/commands.md`, `ai/rules/repo-maintenance.md` | Test-first, how to run commands, what a new tool or gate owes |

## Traps

- **A hook pattern matched a prefix.** `gitVerbRun` (`hookruntime/bash.go`)
  refused `git merge-base` as `git merge`
  (`plan/journal/hook-existing-patterns-false-positive.md`, 2026-09-02).
- **A guard blocked its own author's repair**: the test-weakening check refused
  edits to an untracked new test file
  (`plan/journal/guard-blocks-its-own-authors-repair.md`).
- **Sessions corrupt each other's work in the shared tree**
  (`plan/journal/concurrent-session-corruption.md`). A tree-wide `gofmt -w`
  reformatted another session's files (`ai/META-DECISIONS.md`).
- **A relative scratch path resolved against the package directory**, so a
  `go test` left `proof.json` inside `internal/le/interoplab/bgp/`
  (`plan/journal/test-artifacts-land-in-the-repository-root.md`, 2026-10-09).
  Stray `.go` files in `tmp/` were compiled by `go test ./...`
  (`plan/journal/stray-go-files-in-tmp.md`).
- **Parallel runs collide on a fixed port.** The QEMU SSH port
  probe missed an IPv4 holder on macOS
  (`plan/journal/parallel-copies-collide-on-a-deterministic-port.md`, 2026-10-11).
- **The runner can make a test vacuous.** Its own stdin rewrite in
  `runOneCommand` (`internal/test/runner/parsing.go`) made a TACACS+ security
  `.ci` pass with no config read
  (`plan/journal/green-that-could-not-have-been-red.md`, 2026-09-06).
- **The commit script read the session identity from its runtime environment**,
  and refused a review recorded under another
  (`plan/journal/delegation-hands-the-callee-a-different-argv.md`, 2026-09-05).
- **A wave of unrelated red is a full disk first** (`plan/journal/full-disk-false-red.md`,
  and `./le scratch clean`, `AGENTS.md`).

## Gates

| Command | Judges |
|---------|--------|
| `./le ai hooks unit` | Every hook golden row and behavioral fixture, in process |
| `./le job run label unit-pkg command go test -tags ... ./internal/le/<area>/` | The package you changed, admitted and logged |
| `./le test functional runner` | The runner primitives (`test/runner/*.ci`) |
| `./le cli grammar` | le's namespaces obey the command grammar |
| `./le repo bootstraps`, `./le repo compiles` | le builds from what git holds. The tree compiles in every shipped flavor |
| `./le go vet-platforms darwin` | The shipped tree still compiles on the owner's Mac |
| `./le test weakened selftest` | The weakening detector refuses an unapproved weakening and accepts it once a `ze-test-judge` approval is recorded (`./le test weakened approve`) |
| `./le doc check verify` | Pages and source anchors after a doc edit |
