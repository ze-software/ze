# 1243 -- fixit-ci-accept-only-tests

## Context

~118 of ~1,429 functional `.ci` tests asserted only `expect=exit:code=0` with no value
readback, no `reject=`, and no `set -e` self-checking script. Such a test proves a config
was ACCEPTED, not that it parsed to the CORRECT tree: a parser that accepts `interval 300`
but stores 0, or silently drops a block, still passes green. This is the functional-suite
analog of the "count-only assertion" mistake class. The goal was to strengthen the canonical
examples with a real readback, install a lint that stops the class growing, and grandfather
the existing set so no flag-day backfill is required.

## Decisions

- **The lint is a Go predicate + gate test in `internal/test/runner/`, not a Python script.**
  `accept_only.go` `isAcceptOnly` reuses the real `.ci` parser (`parseAndAdd`) so there is
  exactly ONE definition of "weak" shared by the lint and the baseline generator; a Python
  scanner would re-implement `.ci` parsing. `TestCIAcceptOnlyLint` runs under `ze-unit-test`
  (already a `ze-test`/`ze-verify` dependency, `internal/test/runner` is in `ZE_PACKAGES`), so
  no new `mk/` gate wiring was needed.
- **Ratchet baseline, not a full backfill.** `test/.accept-only-baseline` grandfathers the 100
  currently-weak parseable tests (same shape as `test/.ci-sleep-baseline` /
  `plan/.citation-baseline`, `merge=union` in `.gitattributes`); the gate fails on a NEW
  unannotated weak `.ci` AND on a STALE baseline entry (strengthened/annotated/removed), so the
  ratchet only shrinks. The remaining ~116 are follow-up, not an atomic requirement.
- **Strengthen via `ze config dump --json -`, not the human dump.** Design investigation
  (A-1) found the human `ze config dump -` REFUSES a bgp-less config (`missing required bgp { }
  block`, `cmd_dump.go`) and omits the `environment` subtree; the `--json` form renders it. The
  strengthened `ntp`/`geodns` fixtures keep the original `ze config validate -` step on the
  ntp-only input and ADD a second `ze config dump --json -` step against a bgp-augmented block,
  preserving what the fixture proves today.
- **Annotation escape hatch:** `# accept-only: <reason>` allowlists a deliberately-weak test
  whose value a unit test already covers (AC-3 example: `md5.ci` cites `TestParsePeerMD5FieldsParsed`).

## Gotchas

- **`contains=` truncates its value at the first colon; use `pattern=` for JSON needles.**
  `ParseKVPairs` (`internal/test/ci/ciformat.go`) only preserves colons for the complex keys
  `json=`/`text=`/`hex=`/`pattern=`; a `contains="interval": "300"` silently degrades to matching
  just `"interval"` (non-discriminating -- a stored 0 still prints `"interval"`). Colon-bearing
  readback needles MUST use `pattern=` (real regex, colon-safe). Corollary caution: a `pattern=`
  value that itself contains the literal `json=`/`text=`/`hex=` substring is mis-split by the
  first-marker `strings.Index` -- keep needles free of those substrings.
- **Two fail-open holes an independent review caught (both fixed):** (1) `isAcceptOnly` first
  gated only on the FILE-LEVEL `ExpectExitCode`, so a test asserting only a per-command
  `cmd=...:exit=0` (which `record.go` actively steers multi-`validate` tests toward) escaped
  classification as weak. Fixed to accept ANY zero exit assertion (file or per-command) as the
  acceptance signal, with any non-zero exit → strong. (2) `.ci` files the generic parser cannot
  read were silently skipped with only a `t.Logf` on a passing test -- ~80 files (all
  `test/decode/`, all `test/exabgp-compat/encoding/`, one deliberately-red plugin test) outside
  the ratchet. Fixed to FAIL the gate on any parse error NOT in a documented dialect allowlist
  (`acceptOnlyExcludedParse*`), so a new unparseable exit-only test cannot hide.
- **A guard that skips-and-logs on a passing test is fail-open.** The lesson generalizes: when a
  gate cannot classify an input, it must FAIL or speak loudly, never silently drop it (Ze's
  fail-closed-guards rule). The `t.Logf`-on-pass pattern reads as coverage while providing none.
- The baseline is "100 PARSEABLE weak files"; the dialect allowlist makes the unparseable
  remainder explicit rather than invisible.

## Files

- internal/test/runner/accept_only.go (NEW: predicate, annotation, baseline loader/generator, dialect allowlist, ratchet)
- internal/test/runner/accept_only_lint_test.go (NEW: Flags, per-command-exit, Allows, fail-closed-unparseable, stale-baseline, real-tree gate, regenerator)
- test/.accept-only-baseline (NEW: 100 grandfathered weak tests), .gitattributes (merge=union)
- test/parse/ntp-config.ci, test/parse/geodns-config.ci (strengthened with --json readback), test/parse/md5.ci (AC-3 annotation)
- docs/architecture/testing/ci-format.md (accept-only marker + readback pattern + contains/pattern caution)
- internal/test/runner/record_parse.go (Related back-ref), ai/CODE-TO-DOCS.md, ai/DOCS-TO-CODE.md (regenerated)
