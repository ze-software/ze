# Testing

**When:** writing, changing, or deleting any test, and before writing implementation code for new behavior
**Severity:** blocking
**Related:** completion, platform-linux, rfc-compliance

## Directives

- **A test MUST exist and MUST go red before the code that satisfies it is written.** A test that passes the moment it is written proves nothing about the code, so it MUST be strengthened until it fails.
- **A red test means the code is wrong. MUST NOT weaken, skip, retarget, or delete a test to reach green, and MUST ask the user before deleting or weakening any `*_test.go`, `.ci`, or `.et` content.**
- **A change MUST NOT be claimed done on unit tests alone.** A unit test proves the logic; only a `.ci` or `.et` proves the daemon exposes the behavior through the entry point an operator uses. Which suite runs which format, and what each one asserts, is `docs/functional-tests.md`.
- **A test that cannot run everywhere MUST carry `//go:build linux` on its file or `t.Skip` with a reason, and its assertion MUST NOT be widened to accept both outcomes.**

- **A `.ci` MUST be written and iterated in `test/draft/<suite>/`, and a live one MUST NOT be edited in place.** `test/<suite>/` runs on every verify in this checkout, including runs by other sessions, who then have to work out whether your half-written test is their regression.
- **The draft workflow MUST end in a promotion or a deletion.** The incubator is gitignored and skipped by every repo-wide gate, so nothing in it proves anything, and a session that finds one cannot tell abandoned scaffolding from work in progress. The commands are `docs/functional-tests.md`, "Writing a Test: Draft First".

## Fix Code, Not Tests

- **When a test fails, the code MUST be fixed, and the test's expectations MUST NOT be weakened, simplified, or retargeted to match it.** When the mechanism underneath changes, the expectation stays and the replacement mechanism satisfies it.
- **Test data is covered too: a golden file, an expected output, a fixture, a `.ci` expectation MUST NOT be updated to turn a red run green without the user's explicit approval**, however plausible the new output looks.

- **Two weakenings pass the gate and MUST be judged by hand.** `writeWeakening` reads structure, so it sees neither an expected value changed in place (`Equal(t, 1, x)` to `Equal(t, 2, x)`) nor a rewrite that repoints an existing test at new behavior: the function count and the assertion count are unchanged, and the coverage loss is semantic.
- **A new behavior MUST get a new case, and an existing test MUST NOT be repurposed to carry it.** The behavior that test verified still needs proving.

- **A legitimate weakening MUST have its row written in `test/weakened/<session>.md` before the edit, naming the test this edit weakens, and the commit MUST carry that shard.** The shard is the one your own commit session owns (`./le commit session`), no other session reads it, and the gate drops each row once the commit carrying it lands. The detector reads the shard from disk, so a row written after the refusal opens nothing until the edit is retried, and a row naming another test opens nothing at all. The row format is `docs/architecture/testing/test-health.md`.

## The Affected Population Is Not the Edited Population

- **The tests you write for a change are written against its new contract, so they are green by construction and say nothing about whether the change is safe. The population that can go red is the one written against the old contract, which is exactly the population you did not edit, and it MUST be run before the change is claimed done.** Every gate here scopes itself to `git diff --name-status`, so that population is outside all of them and is yours to derive.
- **When a payload shape changes, you MUST search for the new key name as well as the old one.** Searching what you remove finds code that stops working; it cannot find a branch that already reads the key you added, for a different producer, and now handles your payload wrongly and quietly.

## Proving a Test Discriminates

- **A discrimination proof MUST state whether its re-run actually ran.** `go test` keys a cached verdict on the files the test binary opened, which is narrower than a source hash: a producer the test reaches through `exec`, a compiler it invokes, or an interpreter it shells out to is not one of those files, so mutating it changes no cache key and the tool answers `ok (cached)` for a run that never happened. The tell in the output is a bare `ok` with no duration.
- **A mutation to package source owes nothing further; a mutation to an exec-reached producer MUST defeat the cache with `-count=1`, or drive the producer through a runner that keeps no Go cache, and say which was done.** A `.ci`, `.et`, `.wb` or Docker run has no Go result cache at all, so the caveat MUST NOT be applied where it cannot apply.
- **Applying `-count=1` everywhere MUST NOT be treated as the answer.** It spends the cache of a gate that already costs tens of minutes; the obligation is to know which category the proof is in.

- **New branches owe a measured coverage figure, not an inferred one.** Run the package under `-covermode=count` and read the count for each new outcome. A test list read by name says which behaviors somebody meant to cover.
- **Between the patch and the run, you MUST verify the mutation applied, with a diff that comes back non-empty or a grep for the mutated text.** A patch that fails to apply leaves the test running against unmodified source, so it passes, and the artifact of that attempt is byte-identical to a successful proof. It is the worse half of the trap: a stale cached verdict at least ran once against real code.
- **Restore by copying back a pristine copy saved first; `git checkout --`, `git restore` and `git stash` are banned** and would discard another session's uncommitted work in the same file.

**An applied discrimination cut MUST carry `// MUTATION-APPLIED` and MUST NOT reach a commit; a discrimination note recording which break would redden a test MUST carry `// MUTATION:` and belongs at HEAD.** The two are opposite states wearing one word today. A note is prose above a test naming the break that proves it, which is what a tagged test owes (`ai/rules/interop-and-goal-validation.md`). An applied cut is an edit to product code that makes the product wrong on purpose, for the seconds between breaking it and observing the red. Notes at HEAD in `_test.go` files are right to be there; an applied cut that reaches HEAD ships the defect the test was written to catch, with the test green over it.
**A session that stops an agent mid-proof MUST search the tree for an applied cut before it commits anything.** The window between applying a break and observing the red is where an interruption does its damage, and the agent that held the intent is gone. No gate finds an applied cut. A search for the marker MUST cover `_test.go` files too.

## RFC-Tagged Tests

- **A test carrying an `RFC requirement: <id> <polarity>` tag MUST NOT be edited to match the code.** It is the proof behind a public claim in `docs/features/rfc-status.md`, and `./le rfc check` counts it as that proof, so the edit retires the evidence while the claim stays up. Fix your code instead.
- **A weakening row is your own justification and MUST NOT be read as approval here.** Once the user approves, what they approved MUST be recorded before the edit with `./le rfc approve unit <package>.<TestName> reason "<the owner's words>"`, which writes one row into this commit session's `tmp/commit-rfc-approved-<session>.md`; `writeWeakening` and the commit gate both read that file from disk, the commit carries each row it used as an `RFC-approved:` trailer line, and the generated script drops the used rows once the commit lands.

- **Every gated requirement MUST have both a positive and a negative test, and the assertion MUST name the exact outcome rather than a floor.** A negative-only test passes when the code rejects everything and a positive-only test passes when it accepts everything, so only the pair pins behavior to the requirement. `GreaterOrEqual(TreatAsWithdraw)` is also satisfied by `SessionReset`, so it cannot fail when the implementation over-reacts.

- **A tagged test that is added, moved, deleted or re-tagged MUST NOT be followed by a regeneration, and `ai/RFC-REQUIREMENTS.md`, `rfc/requirements/`, `rfc/enrolled.txt`, `rfc/not-enrolled.txt` and `docs/features/rfc-status.md` MUST NOT be committed**: the five are derived and untracked (`internal/le/rfc/register.go`). Writing a tag carrier removes them, a command that names one rebuilds it, and `./le rfc check` reads the summaries and the tags rather than any generated file.
- **Which carrier a tag MAY live in, and what evidence kind and tier it earns, is `docs/contributing/rfc-implementation-guide.md`.** A tier is derived from the carrier and MUST NOT be declared by the test.

- **The `functional/verify` tier MUST be read as "this suite runs when the change set reaches it", and MUST NOT be read as "this `.ci` runs on every gating run".** A gating run selects its suites from the recorded suite map, in full mode as well as changed mode (`docs/architecture/testing/verify-freshness-scope.md`), so a green functional stage is not evidence that one tagged `.ci` executed, and the suite that carries the tag is what you run when you need that evidence. The tier itself stays derived from the `Gating` list in `internal/le/test/functional/suites.go` and never from the map, so no requirement loses a tier when a run rules its suite out.

- **A unit test file whose `RFC requirement:` tags, proof and gap alike, all cite one RFC MUST be named for it, `rfcNNNN_<topic>_test.go`, and a file named for an RFC MUST carry a tag for that RFC or the marker `// RFC naming: untagged -- <reason>`.** The reason MUST be on the marker's line and MUST state what the file tests and why no tag fits, such as a red defect probe or an RFC that states no requirement the test proves. A file whose tags cite two or more RFCs MAY carry any name that claims no RFC it does not tag. `./le rfc check` reports a file that breaks either direction.
- **A misnamed test file MUST be moved with `./le rfc rename from <old> to <new>` and MUST NOT be moved by hand,** because the rename rewrites the discrimination records and audit verdicts keyed by the old path, and a byte-pure move then owes no new record. The prefixes and the repair are "Test file names" in `docs/contributing/rfc-conformance-gates.md`.

## Iteration Workflow

- **A numeric test id is a position, not an identity, so the stable scenario or Go test name MUST be used in any verification command, handover, gate subset, or evidence claim.** The runner's one-based ordinal is a display position over a sorted fixture population, so adding or renaming an earlier fixture silently renumbers every later row. Why a positional name is stable and a positional number is not is `docs/architecture/testing/runner-architecture.md`.

- **A `ze.log.<subsystem>` key in a `.ci` test MUST name a real slog subsystem.** An internal plugin's logger name is `CanonicalSubsystemName` of its registry name (`internal/component/plugin/inprocess.go`), which turns every hyphen into a dot, and `getLogEnv` (`internal/core/slogutil/slogutil.go`) splits the subsystem on `.` only. So a plugin registered `bgp-adj-rib-in` reads `ze.log.bgp.adj.rib.in`; `ze.log.bgp.adj-rib-in` matches no lookup, sets nothing, and leaves the level at the WARN default with no error. A hyphen is legitimate only when that exact subsystem is declared literally in Go. `checkLogSubsystemKeys` (`internal/le/doc/wiring/checks.go`) enforces it.

- **A crash is not the only reproduction, so `./le test stress-repro run` MUST carry its `any-failure` keyword for a load-dependent failure that is not a crash.** By default only a crash signature (panic, `DATA RACE`, runtime error) counts and everything else is discarded down to the last 500 bytes, so an assertion flake exits non-zero, matches nothing, and the run reports "not reproduced" while throwing the evidence away.

- **A no-build stress reproduction tests the isolated binary set it was given, so after changing daemon source you MUST rebuild before trusting its verdict**, otherwise a fixed bug still "reproduces" against the stale binary. Run the owning `./le test functional <suite>` action once; `internal/le/test/functional.Prepare` rebuilds the isolated daemon and runner pair.
- **A flake MUST NOT be hunted by looping `./le test functional gating` or `./le verify worktree`**: use `./le test stress-repro` against the suspected suite.

## CI Sleep Justification

- **A sleep MUST be converted to a deterministic wait whenever a condition exists to wait on** -- `fixture.Poll` around `fixture.Dispatch`, an SDK readiness callback, a context, or a `wait_until` / `dispatch_until` engine step. A duration is what a test writes when it cannot name the condition, so naming that condition is the work.
- **A sleep that stays MUST carry its justification marker, in the form `// sleep(<kind>): <reason>`, and the reason MUST name a mechanism a later reader can check and overturn.** Two producers enforce it: `./le doc wiring` at gate time and the Write/Edit hook at edit time. The closed set of kinds, what each reason owes, where the comment goes, and the ratchet that caps how many sleeps exist are `docs/architecture/testing/ci-format.md`.

## Temporary Files

- **A scratch file MUST go under this session's own directory, and the system `/tmp` MUST NOT be used.** `dir=$(./le session scratch ensure)` prints the `scratch/` subdirectory of `tmp/session/<YYYY-MM-DD>-<session-id>/`. A fixed name at the `tmp/` root names the same file for every session in the checkout. `bashScratch` and the Write/Edit scratch check in `internal/le/hookruntime` refuse that path.

## Native Test Actions

- **A compiled observer MUST report an assertion failure by returning an error, and MUST NOT print a line and return `nil`.** `fixture.Observe` can still request a clean daemon shutdown, so `expect=exit:code=0` does not prove the observer's assertion and MUST NOT be relied on alone. `fixture.Run` passes the returned error to `fixture.ReportFailure`, which emits the `ZE-OBSERVER-FAIL` sentinel the runner detects.
- **An assertion on a production log line SHOULD be preferred over either**, because it verifies the production code path rather than the observer: `expect=stderr:pattern=<decision log>` plus `reject=stderr:pattern=<wrong outcome>`.

- **A commit owes the focused test for what it changed, run once; the full gate is owed before a push.** That focused test MUST run through a native action: `./le job run label unit-pkg quiet command go test <package>`, a component group (`./le test unit bgp`), or `./le test unit all`. Everything after `command` is the child's argv unchanged, so the `PKG=` spelling belongs to `./le test fuzz` and `go test` refuses it as an import path.
- **A bare `go test` MUST NOT be used in its place.** `internal/le/go/toolchain.Toolchain` gives native actions the repository build cache and the feature tags, and a shell run has neither.

**The action or page in the row MUST be used; the obligation is derivable but the
name is not, and a hand-written second copy of it drifts.**

| Situation | Action or page |
|-----------|----------------|
| Which suite runs which format, and what each `test/<subdir>/` asserts | `docs/functional-tests.md` |
| `.ci` directives, sleep kinds, the sleep ratchet, the accept-only baseline | `docs/architecture/testing/ci-format.md` |
| Any change to `//go:build linux` code | `./le test qemu all-tests` |
| Changes to nft, FIB, or OSPF kernel programming | `./le test qemu netns-test suites firewall,policy,ospf,ospfv3` |
| Build tags, virtual substitutes, and native action wiring for Linux-only code | `ai/rules/platform-linux.md`, read in full first |
| A change to reactor lock or shared state (`session*.go`, `forward_pool*.go`, `peer.go`, a new goroutine there) | `go test -race -count=20 ./internal/component/bgp/reactor/...`, and paste the output as the evidence |
| A VPP backend's Apply pipeline | the `vppOps` seam and scripted `fakeOps` tests, never a running VPP daemon (`internal/plugins/traffic/vpp/apply_test.go`) |
| A suite or gate went red | `tmp/ze-verify-failures.log`, then that group's `Rerun` command and nothing wider |
| Whether the suite is healthy enough to claim it | `docs/features/test-health.md`, generated by `./le test health update` |
| Reproducing a load-dependent failure | `./le test stress-repro` against the suspected suite |
