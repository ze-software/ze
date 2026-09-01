# Testing

**When:** writing, changing, or deleting any test, and before writing implementation code for new behavior
**Severity:** blocking
**Related:** completion, platform-linux, rfc-compliance

## Directives

- **A test MUST exist and MUST go red before the code that satisfies it is written.** A test that passes the moment it is written proves nothing about the code, so it MUST be strengthened until it fails.
- **A red test means the CODE is wrong. MUST NOT weaken, skip, retarget, or delete a test to reach green, and MUST ask the user before deleting or weakening any `*_test.go`, `.ci`, or `.et` content.**
- **A change MUST NOT be claimed done on unit tests alone.** A unit test proves the logic; only a `.ci` or `.et` proves the daemon exposes the behavior through the entry point an operator uses. Which suite runs which format, and what each one asserts, is `docs/functional-tests.md`.
- **A test that cannot run everywhere MUST carry `//go:build linux` on its file or `t.Skip` with a reason, and its assertion MUST NOT be widened to accept both outcomes.**

- **A `.ci` MUST be written and iterated in `test/draft/<suite>/`, and a live one MUST NOT be edited in place.** `test/<suite>/` runs on every verify in this checkout, including runs by other sessions, who then have to work out whether your half-written test is their regression.
- **The draft workflow MUST end in a promotion or a deletion.** The incubator is gitignored and skipped by every repo-wide gate, so nothing in it proves anything, and a session that finds one cannot tell abandoned scaffolding from work in progress. The commands are `docs/functional-tests.md`, "Writing a Test: Draft First".

## Fix Code, Not Tests

- **When a test fails, the CODE MUST be fixed, and the test's expectations MUST NOT be weakened, simplified, or retargeted to match it.** When the mechanism underneath changes, the expectation stays and the replacement mechanism satisfies it.
- **Test data is covered too: a golden file, an expected output, a fixture, a `.ci` expectation MUST NOT be updated to turn a red run green without the user's explicit approval**, however plausible the new output looks.

- **Two weakenings pass the gate and MUST be judged by hand.** `writeWeakening` reads structure, so it sees neither an expected value changed in place (`Equal(t, 1, x)` to `Equal(t, 2, x)`) nor a rewrite that repoints an existing test at new behavior: the function count and the assertion count are unchanged, and the coverage loss is semantic.
- **A new behavior MUST get a NEW case, and an existing test MUST NOT be repurposed to carry it.** The behavior that test verified still needs proving.

- **A legitimate weakening MUST have its row written in `test/weakened.md` BEFORE the edit, naming the test THIS edit weakens, and the commit MUST carry the file.** The detector reads the file from disk, so a row written after the refusal opens nothing until the edit is retried, and a row naming another test opens nothing at all. The row format is `docs/architecture/testing/test-health.md`.

## The Affected Population Is Not the Edited Population

- **The tests you write for a change are written against its NEW contract, so they are green by construction and say nothing about whether the change is safe. The population that CAN go red is the one written against the OLD contract, which is exactly the population you did not edit, and it MUST be run before the change is claimed done.** Every gate here scopes itself to `git diff --name-status`, so that population is outside all of them and is yours to derive.
- **When a payload SHAPE changes, you MUST search for the NEW key name as well as the old one.** Searching what you remove finds code that stops working; it cannot find a branch that already reads the key you added, for a different producer, and now handles your payload wrongly and quietly.

Measured on 2026-08-22: `clear_debt` (`internal/le/commit`) changed the argument its `GateRunner` receives from the repo root to the throwaway worktree. Four new tests were green and six existing `TestDebtClear` cases were red, two of them a genuine semantic break. Measured again on 2026-08-23, when `show bgp rib` moved to flat rows: `internal/component/lg` was never edited, and its `extractRoutes` captured the new shape and returned rows unnormalized, so the looking-glass graph answered `No routes found`, which reads as a true answer about an empty RIB.

## Proving a Test Discriminates

- **A discrimination proof MUST state whether its re-run actually ran.** `go test` keys a cached verdict on the files the TEST BINARY OPENED, which is narrower than a source hash: a producer the test reaches through `exec`, a compiler it invokes, or an interpreter it shells out to is not one of those files, so mutating it changes no cache key and the tool answers `ok (cached)` for a run that never happened. The tell in the output is a bare `ok` with no duration.
- **A mutation to PACKAGE SOURCE owes nothing further; a mutation to an exec-reached producer MUST defeat the cache with `-count=1`, or drive the producer through a runner that keeps no Go cache, and say which was done.** A `.ci`, `.et`, `.wb` or Docker run has no Go result cache at all, so the caveat MUST NOT be applied where it cannot apply.
- **Applying `-count=1` everywhere MUST NOT be treated as the answer.** It spends the cache of a gate that already costs tens of minutes; the obligation is to know which category the proof is in.

- **Between the patch and the run, you MUST verify the MUTATION APPLIED, with a diff that comes back non-empty or a grep for the mutated text.** A patch that fails to apply leaves the test running against unmodified source, so it passes, and the artifact of that attempt is byte-identical to a successful proof. It is the worse half of the trap: a stale cached verdict at least ran once against real code.
- **Restore by copying back a pristine copy saved first; `git checkout --`, `git restore` and `git stash` are banned outright** and would discard another session's uncommitted work in the same file.

## RFC-Tagged Tests

- **A test carrying an `RFC requirement: <id> <polarity>` tag MUST NOT be edited to match the code.** It is the proof behind a public claim in `docs/features/rfc-status.md`, and `./le rfc check` counts it as that proof, so the edit retires the evidence while the claim stays up. Fix your code instead.
- **A row in `test/weakened.md` is your own justification and MUST NOT be read as approval here.** Once the user approves, what they approved MUST be written as one row in `test/rfc-changed.md` before the edit; `writeWeakening` and the commit gate both read that file from disk.

- **Every gated requirement MUST have BOTH a positive and a negative test, and the assertion MUST name the EXACT outcome rather than a floor.** A negative-only test passes when the code rejects everything and a positive-only test passes when it accepts everything, so only the pair pins behavior to the requirement. `GreaterOrEqual(TreatAsWithdraw)` is also satisfied by `SessionReset`, so it cannot fail when the implementation over-reacts.

- **After adding, moving, deleting, or re-tagging a tagged test, or after an edit shifts its line, `./le rfc index-update` MUST run and BOTH of its outputs MUST land in the SAME commit**: `ai/RFC-REQUIREMENTS.md` and every changed file under `rfc/requirements/`. The per-RFC file records each test's `file:line`, and `./le rfc check` fails on a stale index AND on a stale per-RFC file, so committing the index alone lands on the next session as a red gate.
- **Which carrier a tag MAY live in, and what evidence kind and tier it earns, is `docs/contributing/rfc-implementation-guide.md`.** A tier is derived from the carrier and MUST NOT be declared by the test.

## Iteration Workflow

- **A numeric test id is a position, not an identity, so the stable scenario or Go test name MUST be used in any verification command, handover, gate subset, or evidence claim.** The runner's one-based ordinal is a display position over a sorted fixture population, so adding or renaming an earlier fixture silently renumbers every later row. Why a positional NAME is stable and a positional number is not is `docs/architecture/testing/runner-architecture.md`.

- **A `ze.log.<subsystem>` key in a `.ci` test MUST name a real slog subsystem.** An internal plugin's logger name is `CanonicalSubsystemName` of its registry name (`internal/component/plugin/inprocess.go`), which turns every hyphen into a dot, and `getLogEnv` (`internal/core/slogutil/slogutil.go`) splits the subsystem on `.` only. So a plugin registered `bgp-adj-rib-in` reads `ze.log.bgp.adj.rib.in`; `ze.log.bgp.adj-rib-in` matches no lookup, sets nothing, and leaves the level at the WARN default with no error, which is why it has recurred three times. A hyphen is legitimate ONLY when that exact subsystem is declared literally in Go. `checkLogSubsystemKeys` (`internal/le/doc/wiring/checks.go`) enforces it.

- **A crash is not the only reproduction, so `./le stress-repro run` MUST carry its `any-failure` keyword for a load-dependent failure that is not a crash.** By default only a crash signature (panic, `DATA RACE`, runtime error) counts and everything else is discarded down to the last 500 bytes, so an assertion flake exits non-zero, matches nothing, and the run reports "not reproduced" while throwing the evidence away.

- **A no-build stress reproduction tests the isolated binary set it was given, so after changing daemon source you MUST rebuild before trusting its verdict**, otherwise a fixed bug still "reproduces" against the stale binary. Run the owning `./le functional <suite>` action once; `internal/le/functional.Prepare` rebuilds the isolated daemon and runner pair.
- **A flake MUST NOT be hunted by looping `./le functional` or `./le verify worktree`**: use `./le stress-repro` against the suspected suite.

## CI Sleep Justification

- **A sleep MUST be converted to a deterministic wait whenever a condition exists to wait on** -- `fixture.Poll` around `fixture.Dispatch`, an SDK readiness callback, a context, or a `wait_until` / `dispatch_until` engine step. A duration is what a test writes when it cannot name the condition, so naming that condition is the work.
- **A sleep that stays MUST carry its justification marker, in the form `// sleep(<kind>): <reason>`, and the reason MUST name a mechanism a later reader can check and overturn.** Two producers enforce it: `./le doc wiring` at gate time and the Write/Edit hook at edit time. The closed set of kinds, what each reason owes, where the comment goes, and the ratchet that caps how many sleeps exist are `docs/architecture/testing/ci-format.md`.

## Temporary Files

- **A scratch file MUST go under this session's own directory, and the system `/tmp` MUST NOT be used.** `dir=$(./le session scratch ensure)` prints the `scratch/` subdirectory of `tmp/session/<YYYY-MM-DD>-<session-id>/`. A fixed name at the `tmp/` root is the failure this replaces: it names the same file for every session in the checkout. `bashScratch` and the Write/Edit scratch check in `internal/le/hookruntime` refuse that path.

## Native Test Actions

- **A compiled observer MUST report an assertion failure by RETURNING an error, and MUST NOT print a line and return `nil`.** `fixture.Observe` can still request a clean daemon shutdown, so `expect=exit:code=0` does not prove the observer's assertion and MUST NOT be relied on alone. `fixture.Run` passes the returned error to `fixture.ReportFailure`, which emits the `ZE-OBSERVER-FAIL` sentinel the runner detects.
- **An assertion on a production log line SHOULD be preferred over either**, because it verifies the production code path rather than the observer: `expect=stderr:pattern=<decision log>` plus `reject=stderr:pattern=<wrong outcome>`.

- **A commit owes the focused test for what it changed, run once; the full gate is owed before a PUSH.** That focused test MUST run through a native action: `./le job run label unit-pkg quiet command go test <package>`, a component group (`./le test-unit bgp`), or `./le test-unit`. Everything after `command` is the child's argv unchanged, so the `PKG=` spelling belongs to `./le fuzz` and `go test` refuses it as an import path.
- **A bare `go test` MUST NOT be used in its place.** `internal/le/gotoolchain.Toolchain` gives native actions the repository build cache and the feature tags, and a shell run has neither.

**The action or page in the row MUST be used; the obligation is derivable but the
NAME is not, and a hand-written second copy of it drifts.**

| Situation | Action or page |
|-----------|----------------|
| Which suite runs which format, and what each `test/<subdir>/` asserts | `docs/functional-tests.md` |
| `.ci` directives, sleep kinds, the sleep ratchet, the accept-only baseline | `docs/architecture/testing/ci-format.md` |
| Any change to `//go:build linux` code | `./le qemu all-tests` |
| Changes to nft, FIB, or OSPF kernel programming | `./le qemu netns-test suites firewall,policy,ospf,ospfv3` |
| Build tags, virtual substitutes, and native action wiring for Linux-only code | `ai/rules/platform-linux.md`, read in full first |
| A change to reactor lock or shared state (`session*.go`, `forward_pool*.go`, `peer.go`, a new goroutine there) | `go test -race -count=20 ./internal/component/bgp/reactor/...`, and paste the output as the evidence |
| A VPP backend's Apply pipeline | the `vppOps` seam and scripted `fakeOps` tests, never a running VPP daemon (`internal/plugins/traffic/vpp/apply_test.go`) |
| A suite or gate went red | `tmp/ze-verify-failures.log`, then that group's `Rerun` command and nothing wider |
| Whether the suite is healthy enough to claim it | `docs/features/test-health.md`, generated by `./le test-health update` |
| Reproducing a load-dependent failure | `./le stress-repro` against the suspected suite |
