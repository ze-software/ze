# 1197 -- fixit: agent-facing gates that lied to the agent

## Context

Six agent-facing surfaces stated a requirement their own implementation did not
honour: an agent obeying the text was punished by the tool, an agent obeying the
tool violated the text. All six were verified at the producer (not inferred from
the doc) and filed in `plan/learned/HOOK-FRICTION.md` (F2/F3/F4 + T-4/T-5/T-6
found while writing/committing the spec). Spec:
`plan/spec-fixit-agent-tooling-misleads.md`. The unifying constraint: each gate
is CORRECT in what it enforces; only the mechanics lied or over-narrowed. Every
fix ACCEPTS the mandated/correct form without accepting prose (AC-7: no gate
enforces less than before), each paired with a must-not-fire test.

## Decisions

- **T-1 (`.claude/hooks/validate-spec.sh`): the Current Behavior citation regex
  required the backticked path to END in `go|py|rs|ts|js`.** A trailing `:line`
  (the exact form `ai/rules/no-fabrication.md` mandates) defeated the match, and
  `.sh`/`Makefile` were absent so a shell spec could not cite its own subject.
  New path atom: `` `[^`]*(\.(go|py|rs|ts|js|sh|mk)|Makefile)(:[0-9]+)?` ``. The
  `- [ ]`/`- [x]` checkbox anchor already keeps prose out, so widening the atom
  does not accept sentences. Also removed the `| head -30` window: a long
  section's citation past line 30 was invisible and wrongly demanded.
- **T-5 (same file): the `.ci` functional-test demand assumed every spec is a
  daemon feature.** A `.ci` drives the ze daemon; hooks/dev-scripts never touch
  one. Scoped the demand to specs whose `## Files to Modify` names daemon Go
  (`internal/*.go` or `cmd/*.go`); a tooling-only spec may instead name a
  concrete `.py`/`.sh` driving surface. Daemon specs STILL require a `.ci` (the
  rule exists because unit tests let 30 tests ship binding no peer, `aaefef8ce`).
- **T-4 real producer is `.claude/hooks/mark-source-read.sh`, not
  `pretool-writeedit.py`.** The spec named pretool-writeedit for "the evidence
  set", but that check only READS the `.source-read-<sid>` marker; the shell
  `case` in mark-source-read.sh is what WRITES it. Widened that case to `.py`
  under `scripts/`, `.sh` under `.claude/hooks/`, `Makefile`, `mk/`, and updated
  the pretool message text to match (truthfulness). Edited the producer per
  `ai/rules/no-fabrication.md`; docs/specs/unrelated files still skip the marker.
- **T-2 (`.claude/rules/session-start.md`): doc carve-out, no gate change.**
  Verified at the producer that `block-until-lsp.sh:36` already lifts on the
  ToolSearch QUERY TEXT (`grep -qi LSP`), not on a successful load (deliberate,
  comment `:32-35`). A PreToolUse hook structurally cannot see ToolSearch
  RESULTS, so the only owed fix is to stop the banned-excuses table telling
  subagents they must LOAD a tool their harness does not expose. Added a
  subagent carve-out: issuing the query and getting "No matching deferred tools
  found" SATISFIES step 1.
- **T-3 (`scripts/dev/commit_helper.py` + `ai/rules/error-messages.md`): a
  remediation that cannot work.** The structural-gate refusal told the reader to
  re-run `make <gate>` (e.g. `ze-lint-changed`) to "refresh
  tmp/ze-verify-failures.json". Verified only `scripts/status/verify_run.go`
  (via `make ze-verify`/`ze-verify-changed`) writes that file; a lint target
  does not. Fixed the message to name the true refresher and say the per-gate
  command does NOT refresh. Generalised in `error-messages.md`: leg-3 advice
  must be verifiably TRUE (read the producer of the promised effect first).
- **T-6 (`scripts/dev/discovery_sources.py` + `commit_helper.py`): the index
  gate demanded indexes the commit does not feed.** `OUTPUTS` was a flat tuple
  and `is_discovery_source` a bare bool ("source of ANY index"), so a commit
  adding a learned summary (feeds LEARNED-FULL-INDEX) was refused for an
  unrelated dirty `ai/DOCS-TO-CODE.md` -- whose only remediation cross-commits
  another session's row. Turned the docstring's prose into DATA: added
  `indexes_fed_by(path, header)` returning the subset of `OUTPUTS` a source
  feeds (committed index -> itself; Makefile/mk -> all; generator -> its paired
  output; `plan/learned/*` -> LEARNED; `register.go`/`// Package` -> PACKAGE-MAP;
  `// Design:` -> DOCS-TO-CODE). `discovery_index_problems` now demands only the
  indexes THIS commit's sources feed. `is_discovery_source` reimplemented as
  `bool(indexes_fed_by)` so the "is-it-a-source" and "which-index" answers can
  never disagree. Kept ONE `--stale-index-ok` flag: per-index scoping deletes
  the "someone else's dirty index" case at the gate, so no second override
  spelling is needed. A genuinely fed-but-omitted stale index STILL refuses.

## Gotchas

- `python3 scripts/dev/learned_index.py --help` REGENERATES `ai/LEARNED-FULL-INDEX.md`
  as a side effect (no dedicated help path). Use `--check` to test freshness.
- The spec's own tooling found T-5 and T-6 while writing/committing it: the
  count grows because the surfaces are used, not audited.

## Tests

- `scripts/dev/hook-fixture-check.py`: `validate-spec` section (T-1 line-numbered
  / shell / Makefile citations accepted, whole-section read, prose still
  rejected; T-5 tooling surface accepted, daemon still needs `.ci`) and a new
  `mark-source-read` section (T-4 marker written for .py/.sh/Makefile/mk, skipped
  for doc/spec/unrelated).
- `scripts/dev/discovery_sources_test.py`: `TestIndexesFedBy` (per-index map).
- `scripts/dev/commit_helper_test.py`: `TestDiscoveryIndexProblems` (unrelated
  dirty index PASSES, fed-but-omitted stale index REFUSES) and
  `TestStructuralGateRemediation` (AC-5 refusal names the true refresher).

## Files

None recorded.
