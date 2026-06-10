# Handover: follow-ups discovered during the 2026-05-31 review-fix pass

The review-fix commit resolved every BLOCKER/ISSUE/actionable-NIT found while
critically reviewing the 2026-05-30 and 2026-05-31 commits. While verifying
those fixes, several **pre-existing, unrelated** problems surfaced. They are NOT
caused by the review-fix commit; they were hidden (mostly by a dead test suite)
and are now visible. This handover enumerates them for a follow-up pass.

Verification status at handover: `make ze-lint-changed` = 0 issues; every package
touched by the review-fix commit passes its own tests (the `rib` package green
under `-race`, including a new concurrent show+churn test). `make ze-verify` was
NOT run to completion (interrupted; the failures below are the known reason).

---

## 1. UI test suite was undiscoverable since `c04d2f249` (root cause, now unblocked)

`test/ui/doctor-gokrazy-update-check.ci` used `reject=stdout:`, which the
EncodingTests runner errored on (`unknown reject type "stdout"`), and that error
**aborted discovery of the entire `test/ui` suite**. So `ze-test ui -a` failed at
discovery and ran zero tests, ever since `c04d2f249`. The review-fix commit added
`reject=stdout` / `expect=stdout:!contains` support, so the suite discovers again.

**Residual fragility (FIX RECOMMENDED):** one unparseable `.ci` still aborts the
whole suite. `internal/test/runner` discovery should skip-and-warn on a parse
error rather than abort, so a single bad file cannot hide the rest again.

## 2. Nine pre-existing UI test failures, now visible (TRIAGE, ideally on Linux CI)

`ze-test ui -a` on macOS: 118/127 pass, 8 skipped, 9 fail. None of the 9 use the
new `!contains`/`reject=stdout` features, so the review-fix commit did not cause
them. Several look macOS-vs-Linux environment specific and may pass on the Linux
CI target. Observed reasons:

| ID  | Test                          | Observed failure |
|-----|-------------------------------|------------------|
| 36  | cli-completion-option-targets | stdout does not contain "blame" (completion drift) |
| 91  | doctor-aaa-unreachable        | exit_code_mismatch (expected 0, got 1) |
| 100 | doctor-listeners              | exit_code_mismatch (likely host listener/port env) |
| 105 | doctor-platform               | TYPE "unknown" -> see footgun #3 (no `expect:exit`) |
| 129 | support-exclude               | stderr does not contain "mutually exclusive" |
| 130 | support-list-modules          | stdout does not contain "version" (no kernel modules on macOS) |
| 131 | support-module-filter         | exit_code_mismatch |
| 132 | support-reason                | exit_code_mismatch |
| 133 | web-commit-reject             | http_check_failed |

Action: re-run `ze-test ui -a` on Linux CI, then fix the genuine (non-env)
failures. Add proper `skip-os` guards to any that are Linux-only.

## 3. Harness footgun: stdout assertions only run when `expect:exit` is present

`internal/test/runner/runner_exec.go:907` wraps the stdout/stderr/logging/file
checks in `if rec.ExpectExitCode != nil { ... }`. A `cmd=foreground` `.ci` with
no `expect=exit:code=N` has ALL its assertions silently skipped, then falls
through to a default `stateUnknown` failure. This bit `doctor-plugin-external-builtin.ci`
(fixed in the review-fix commit by adding `expect=exit:code=0`) and is the likely
cause of `doctor-platform` (105) failing as TYPE "unknown".

Action: run the stdout/stderr/file checks regardless of whether an exit
expectation is set (carefully -- this may surface more currently-hidden
failures). Audit `test/ui/*.ci` for foreground commands lacking `expect:exit`.

## 4. `TestCheckListeners_API` naming mismatch (cmd/ze/doctor)

Deterministic failure (fails in isolation, independent of the review-fix commit:
that diff only touches `checkPlugins`). The diagnostic message is
`"api-server-rest s1: cannot bind ..."` but the test asserts the message contains
`"api-rest"`. `doctor.go:949` builds `tcpListener("api-rest", ...)`, yet the
config container in the test is `api-server`, and the runtime name is
`api-server-rest`.

Action: determine the canonical listener name (is `doctor.go:949`'s literal
`"api-rest"` the live path, or is the name derived from the `api-server`
container?). Fix either the code or the test assertion to agree.

## 5. Review items deliberately left (no current bug; decide if worth doing)

- **`7368aa16a` derive-from-dispatch half-met:** `bgp`/`iface`/`schema`/`yang`
  register hardcoded `*Commands` slices "kept in sync with the switch" with no
  parity test. Correct today, drift-prone. Add a slice-vs-switch table test per
  package, or fold the slice into the switch dispatch.
- **`config validate` doesn't run loader internal-plugin checks:** `internal rib {}`
  (missing `use`) and an internal/external duplicate name both pass
  `ze config validate` (exit 0) yet fail at boot. `validate` runs only the
  registry verifiers, not `ExtractPluginsFromTree`. Pre-existing pattern (external
  duplicate has the same gap), but it gives false confidence for the new keyword.
- **Cursor RFC-9494 stale-metadata claim overstated:** `00b7e0ad1` commit message
  and `plan/learned/824-rib-feed-replay-batch.md` say `resendRoutesWithCursor`
  "preserves RFC 9494 metadata", but `CommandContext.Meta` is never consumed by
  the update egress path (documented gap at `pkg/plugin/sdk/sdk_engine.go:24-26`).
  Correct the claim.
- **`.claude/hooks/block-raw-ansi.sh`** does not catch the Go unicode-escape
  form (backslash-u-001b followed by a bracket) -- false negative; only octal
  `\033[` is handled.
- **`internal/component/config/validators.go` `sortedInternalPluginNames`**
  re-sorts an already-sorted, freshly-allocated slice (redundant, no bug).
- **`test/plugin/redistribute-l2tp-not-configured.ci`** still uses the legacy
  `external ... { use ... }` form while its three siblings migrated to the
  `internal` keyword.

## 6. Pre-existing attrpool items (NOT from the reviewed commits; flagged in review)

- **`attrpool` does not compile under `-tags debug`:** `debug_test.go:48` calls
  the old 1-return `Intern`, so the shard-aware debug validators get zero CI
  coverage. (Documented gotcha in `plan/learned/821-spec-attrpool-shard.md`.)
- **`migrateBatch` reuse-during-compaction offset hazard** (`attrpool/pool.go`
  ~715-743): a slot reused via the free list during an active incremental
  compaction can copy stale offset data. Latent (no test drives it; the
  scheduler-driven production path has not surfaced it). The sharding rewrite
  preserved this verbatim rather than introducing it.
