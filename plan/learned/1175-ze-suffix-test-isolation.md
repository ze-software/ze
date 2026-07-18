# 1175 -- ze-suffix-test-isolation

## Context

Functional targets in `mk/test-functional.mk` drove `bin/ze-test`, and the runner
(`internal/test/runner` `Build`) recompiled `ze` + `ze-test` into `bin/` on every run. So a
test run and interactive development fought over `bin/ze`: `make ze` or an edit made while a
suite ran clobbered the dev binary, and half-edited source leaked into later suites. The user
asked for test runs to build a uniquely-named binary so development can continue untouched.

## Decisions

- Made isolation the DEFAULT for every functional target (per-suite and the gating
  `ze-functional-test`, so `ze-verify` inherits it). Each run builds `ze`/`ze-test`/`ze-stripped`
  once into `tmp/testbin-<suffix>/` and runs frozen via `ZE_TEST_NO_BUILD=1` +
  `ZE_BIN`/`ZE_TEST_BIN`; the dir is removed on exit by a shell `trap`.
- Suffix defaults to the make PID (`:= pid-$(shell echo $$PPID)`, immediate expansion so it is
  stable across a run's suites and unique per invocation). `ZE_SUFFIX=<name>` pins a stable,
  kept directory; `ZE_TEST_CANONICAL=1` opts out to the legacy in-place `bin/ze-test` rebuild.
- Binaries carry the canonical names `ze`/`ze-test`/`ze-stripped` directly in the dir (no
  symlinks): `.ci` tests exec `ze`/`ze-stripped` by bare name and the runner puts `ZE_BIN`'s
  directory first on `PATH` (`internal/test/runner/runner_exec.go`).

## Consequences

- `bin/ze` is never touched by a functional test; you can rebuild/edit it while a suite runs.
- Any new functional target here must go through `$(ZE_TEST_RUN)` and carry the
  `@trap '$(ZE_ALT_TRAP)' EXIT; $(ZE_ALT_BUILD)` prefix, or it will not isolate / clean up.
- Interrupted runs (SIGKILL, e.g. a stopped `ze-verify`) leave their `tmp/testbin-*` dir behind
  because the trap cannot fire; `make ze-clean-tmp` sweeps dirs older than 24h.

## Gotchas

- **ze derives its config/DB dir from its own binary location, and only accepts a parent named
  bin/sbin** (`internal/core/paths/paths.go` `isBinDir`/`ConfigDirFromBinary`). A binary built
  directly into `tmp/testbin-<suffix>/` returns "" -> "cannot determine database location", which
  broke `ze config archive` (`test/parse/cli-config-archive.ci`, emitted that instead of the
  expected "no credentials"). Fix: build into a `bin/` SUBDIR (`tmp/testbin-<suffix>/bin/`). Any
  relocation of the ze binary must preserve a `bin`-named parent, or set `ze.config.dir`.
- **A once-per-invocation phony prereq + per-recipe cleanup traps collide.** The first version
  built ONE shared dir via a phony `ze-alt-binaries` prereq, and each recipe had
  `trap 'rm -rf <dir>'`. Chaining targets (`make ze-encode-test ze-plugin-test`, esp. under `-j`)
  let the first target's trap delete the dir mid-run of the second. Fix: build inline per recipe
  into a per-target dir (`...-$@`), so each target owns its dir. Arm the trap BEFORE the build so
  a failed build is cleaned up too.
- **Indirecting a literal token through a make variable breaks parsers that scan for the
  literal.** Replacing `bin/ze-test` with `$(ZE_TEST_RUN)` in the run-suite lines broke
  `scripts/docvalid/doc_drift.go` `zeTestSuiteFromMakeLine`, which derived the release-gate suite
  list by scanning for the field `bin/ze-test`; `ze-doc-test` then failed with "could not derive
  ze-functional-test suites from Makefile". Fix was to teach the parser the new token too. When
  you route a Makefile literal through a variable, grep for tools that read the raw Makefile text.
- `?=` / `=` with a `$(shell)` RHS is recursively expanded, so the shell re-runs on every
  expansion and yields a different PID per use. Use `:=` for the PID (computed once), and reserve
  recursive `=` for `ZE_ALT_DIR`/`ZE_TEST_RUN` only because they must re-expand `$@` per recipe.
- **Attribute a full-suite red before blaming your change.** Running the whole functional suite
  surfaced ~10 daemon-suite failures; the as112 watchdog tests failed IDENTICALLY in canonical
  (`bin/ze-test`) and isolated mode, proving they were pre-existing, and `bfd-echo-handshake`
  passed alone but failed under full-run load (a `flaky-under-load` timing race). Compare the same
  test in both modes at low parallelism before concluding a binary-relocation change caused a
  behavioral failure.
