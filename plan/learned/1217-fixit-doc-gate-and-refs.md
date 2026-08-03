# 1217 -- fixit-doc-gate-and-refs

Wired the DARK documentation-consistency gate into the CI verify pipeline,
fixed the stale discovery-layer references it would have caught, tightened
`doc_drift.go` to see bare/undercount claims, and unified contradictory
headline counts.

## What was dark and why it mattered

`stagesForMode` (`scripts/status/verify_run.go`) is the ACTUAL source of truth
for what `make ze-verify` runs (the dead `_ze-verify-impl` Makefile targets have
zero callers). Neither branch named any doc gate, so `check_doc_links.py` could
exit 1 with broken references IN THE DISCOVERY LAYER ITSELF (index/rule files
agents read to find code) while CI stayed green. The fix is to wire the gate
where CI actually runs it, not a `.claude/hooks/*` (which fires only for Claude,
never for CI or other agents).

## Key decisions / gotchas

- Wire BOTH `ze-doc-test` (doc_drift + anchors) AND `ze-doc-links`
  (`check_doc_links.py`) into BOTH `stagesForMode` branches. `ze-doc-test` does
  NOT run `check_doc_links.py` -- only `ze-doc-links`/`ze-regen-check` do
  (`Makefile:441,472`). Wiring `ze-doc-test` alone would miss every broken path
  ref. Both branches, or `ze-verify-changed` sessions skip the gate (R-3).
- **doc_drift bare-count check**: `checkReadmeCount` captures the optional `+` as
  its own regex group (RE2 has no look-ahead), so one matcher tells at-least
  (`N+`, drift only when actual < N -- soft, R-1 anti-re-drift) from bare exact
  (`N`, drift when actual != N in EITHER direction). AC-3's "undercount" means a
  BARE undercount; `N+` undercounts stay tolerated on purpose (else soft phrasing
  re-drifts as the tree grows).
- **`doc_drift.go` counts differ from a naive grep**: `countFuzzTargets` walks
  the WHOLE tree (327 `func Fuzz`), not just registered targets (~72 in
  `functional-tests.md`); `countGoTestFunctions` walks internal/pkg/cmd (~19k).
  Soft floors (`50+ fuzz`, `10,000+ unit`) satisfy the live counts.
- **readMakefileLines nested-include bug (found via a concurrent uncommitted
  change)**: it resolved a nested `include mk/test-fuzz-targets.mk` relative to
  the including file's dir (`mk/mk/...`), but GNU make resolves includes relative
  to CWD (repo root). The hard include then failed to read and aborted the whole
  Makefile parse, silently emptying the derived ze-functional-test suite list
  ("could not derive ze-functional-test suites from Makefile"). Fixed to resolve
  relative to root. This is doc-gate tooling robustness, independent of the spec's
  ACs but required for the wired gate to parse correctly.
- **`check_doc_links.py` is PER-LINE**: a `doc-links: ignore` marker must sit on
  the SAME line as the broken backtick ref. First attempt put the marker one line
  below the `cmd/{dhcp,ntp,heartbeat,randomd}` refs and it stayed broken.
- **Curated vs generated indexes**: only `ai/LEARNED-FULL-INDEX.md` is generated
  (`scripts/dev/learned_index.py`). `ai/INDEX.md` and `ai/LEARNED-INDEX.md` are
  CURATED/hand-maintained (no script writes them) -- verify at the producer before
  assuming an index is generated. Their stale refs must be hand-fixed; no
  generator can.
- **no-sprintf-alloc hook fires even in `//go:build ignore` build tools**: new
  `fmt.Sprintf("%d...")` lines are blocked. Built the new drift messages with
  `textbuf.Buffer` (`.Str().Int().String()`) instead. `//go:build ignore` files
  are excluded from lint/`go test` compilation, so new funcs there are only
  exercised by the `go run ... --root <tmp>` subprocess smoke tests.

## Reference resolutions (19 refs / 9 files)

Moved paths repointed (`internal/component/bgp/{attribute,capability}` ->
`internal/core/bgp/...`; `internal/plugins/ospfv3/` -> `internal/plugins/ospf/v3/`;
closed `spec-vrrp-0-umbrella` -> the live `internal/plugins/vrrp/`). Intentional
gaps marked `doc-links: ignore` (illustrative spec-name examples, worktree-only
paths, runtime archive dirs, cross-repo gh-pages tooling, module-cache submodule
shorthand, an intentionally-absent deferred `.ci`). Deleted `test/stress/bgpgen.py`
dropped from `repo-maintenance.md` and both `.claude/memory/` files.

## Verification without ze-verify

`make ze-verify` (and `ze-doc-test`/`ze-doc-links`) KILL live servers, so the
wired gate was proven via its composition: AC-1 unit tests show both stages are
in the CI stage list; `check_doc_links.py` and `doc_drift.go` each exit 1 on
injected breakage and 0 when clean (witnessed live).

## Files

None recorded.
