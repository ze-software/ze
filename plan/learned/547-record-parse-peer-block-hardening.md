# 547 -- record_parse.go peer-block directive hardening

## Context

`internal/test/runner/record_parse.go`'s peer-block loop silently dropped every
line that was not `expect=` or `action=`. Test authors placed `option=env:var=X:value=Y`
inside `stdin=peer:...EOF` blocks for visual locality, expecting it to seed the
test-runner environment. In reality the runner handed the line unchanged to
`ze-peer`, which ignored it, so the env var never took effect. At least two
graceful-restart tests (`gr-marker-restart.ci`, `gr-marker-expired.ci`) were
broken-or-passing-for-the-wrong-reason for months before incident 545 surfaced it.
The goal: make the parser reject this at `-list` time with an actionable error,
and audit every `.ci` file in the repo for the same latent bug.

## Decisions

- **Hard error over auto-promotion.** The parser returns `fmt.Errorf(...)` from
  `parseAndAdd` naming the directive, rather than silently lifting it into
  record-level `EnvVars`. Auto-promotion would be helpful-looking behavior that
  obscures intent-vs-placement mismatches — explicit placement is clearer and the
  error at `-list` makes the audit self-surfacing.
- **Scope limited to `option=env`.** Not a broader allow-list of valid peer-block
  directives. Enumerating every directive `ze-peer` consumes would be speculative
  work and risked breaking untested paths. If `cmd=`, `reject=`, `http=` inside
  peer blocks ever get silently dropped, file a new spec.
- **Error message format reused from existing conventions.** `Discover` already
  wraps parse errors with `fmt.Errorf("%s: %w", filepath.Base(ciFile), err)`, so
  the inner error does NOT repeat the filename. The first draft produced
  `logging-level-filter.ci: logging-level-filter.ci: ...` because of the double
  wrap — dropped before running the full audit.
- **`.ci` fix-up preserves intent, not correctness beyond scope.** Moving the
  directive outside the block is the spec's job. Fixing typos in the env var
  *name* is out of scope — see Gotchas.

## Consequences

- Every future `.ci` file with `option=env` inside `stdin=peer` fails at
  `bin/ze-test <suite> -list` with a message quoting the exact directive and
  pointing at `plan/learned/545-debug-plugin-test-cluster.md`. The test cannot
  ship broken — the parse error blocks discovery.
- `docs/functional-tests.md` now has a "Directive Placement" subsection explaining
  which directives belong in which scope (test runner vs `ze-peer` stdin), with
  correct-vs-rejected examples and a `<!-- source: record_parse.go -->` anchor.
- Four plugin tests were edited to move the directive outside the block:
  `logging-level-filter.ci`, `logging-stderr.ci`, `logging-syslog.ci`,
  `metrics-flap-notification-duration.ci`. The other candidates in the original
  grep-based list were already clean — the parser error at `-list` is the
  authoritative audit, not a hand-curated list.
- No functional tests regressed. `make ze-functional-test` and `make ze-verify`
  both pass (8/8 suites, 225/225 plugin tests).

## Gotchas

- **The grep list was noisy.** `grep option=env test/plugin/*.ci` finds every file
  with the directive anywhere, not just files with it *inside* a peer block. Eight
  files matched the grep but only four actually had the bug. Always trust the
  parser error at `-list` time, not a hand-curated list.
- **Latent naming bug in the logging tests.** All three logging `.ci` files
  (`logging-level-filter`, `logging-stderr`, `logging-syslog`) use
  `option=env:var=ze.bgp.log.server:value=...`. The registered convention is
  `ze.log.<subsystem>` (see `internal/core/slogutil/slogutil.go:45` —
  `MustRegister("ze.log.<subsystem>", ...)`). So `ze.bgp.log.server` was being
  dropped twice: first by the parser (this spec's fix), then by the env registry
  on the ze side. The tests pass anyway because their stderr/syslog patterns
  match messages that appear at the default log level. Fixing the typo is left
  for a follow-up spec — verify the assertions still hold with the level actually
  in effect before changing the name.
- **Error wrapping doubles the filename.** `EncodingTests.Discover` wraps every
  `parseAndAdd` error with `filepath.Base(ciFile)`. Adding the filename again
  inside the inner error produces `foo.ci: foo.ci: ...`. Look at adjacent
  `return fmt.Errorf("line %d: %w", ...)` calls as the template — they omit the
  filename deliberately.
- **Spec candidate lists drift.** The spec listed eight candidate `.ci` files
  based on a grep snapshot. Four were already fixed by a prior commit (`c9251a7e`
  for `gr-marker-restart.ci` and `gr-marker-expired.ci`) or simply never had the
  bug (`rs-backpressure.ci`, `gr-cli-restart.ci`, etc.). Re-check before editing.

## Files

- `internal/test/runner/record_parse.go` — peer-block loop rejects `option=env`
- `internal/test/runner/record_parse_test.go` — new; three TDD tests (outside OK,
  inside rejected, timeout/open/update pass-through)
- `docs/functional-tests.md` — new "Directive Placement" subsection with source anchor
- `test/plugin/logging-level-filter.ci`, `logging-stderr.ci`, `logging-syslog.ci`,
  `metrics-flap-notification-duration.ci` — env var moved outside peer block
