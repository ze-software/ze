# 1328 -- Rule corpus merged to 26, and line numbers removed from prose

**Date:** 2026-08-03
**Trigger:** Thomas measured the repository writing more prose than code and asked
for the churn removed. Over 30 commits: 6,167 lines of Go against 24,082 lines of
`docs/` + `ai/` + `plan/`.

## What changed

| Change | Before | After |
|--------|--------|-------|
| `ai/rules/` files | 102 | 29 (26 rules, 3 generated) |
| `ai/rules/` lines | 17,333 | 10,756 |
| `CONDENSED.md` | 5,182 lines, regenerated on every rule edit | deleted; nothing loaded it |
| Line-number citations in prose | 15,708 | 0 path-form, 1,261 bare `` `:N` `` reported |
| Always-loaded session payload | 21,548 tokens | 18,253 tokens |

## Lessons

- **A merge agent can silently drop a source and still report success.** One bucket received only the first of five sources while its own report claimed all five. A directive-line count per bucket caught it (62% against a 95% floor for the others); the prose did not. Count before deleting the sources.
- **Vocabulary coverage is the cheaper second check.** Distinct 5-letter words of each source against the merged text, at a 72% floor, found the same defect and needs no parsing of directive shapes. Run both: the count finds a truncated merge, the coverage finds an omitted source.
- **A generated evidence ledger is not prose and must be exempt from a prose sweep.** `ai/RFC-REQUIREMENTS.md` cites `file.go:line` to name one tagged test unit, so there the line IS the fact. `scripts/dev/line_refs.py` and `c_line_number_ref` both name it. Without the exemption the sweep deletes the evidence the RFC gate reads.
- **`git ls-files` does not list the files a merge just created.** The rename pass ran before the merged rules were tracked, so it repointed 3,148 references and missed the 15 files citing the retired paths most. Use `--cached --others --exclude-standard` whenever a pass must cover work created in the same session.
- **Core membership is derived, so a merge can change it silently.** `quality.md` and `spec-no-code.md` left `CORE.md`: with 26 triggers instead of 98, document frequency fell and the router now reaches both. `make ze-rules-router-report` confirmed real tasks surface them. Read that report after any change to corpus size, and do not read a smaller core as a loss.
- **The RFC audit ledger stores line offsets into test files.** A sibling session's one-line edit to `rfc7606_optional_attrs_test.go` SHIFTED four verdicts whose tagged units are byte-identical. Same rot class as the prose citations, still live in the compliance machinery. `make ze-rfc-reseal` re-stamps it.

## Committing a repo-wide rename in a shared checkout

Replaying the session's own mechanical transforms over each file's HEAD content
attributes the working tree exactly: a file that reproduces byte for byte carries this
session's edit and nothing else. That split 1,434 modified files into 1,298 clean and
136 mixed.

- **For a mixed file whose HEAD cites a retired path, staging the working tree absorbs a sibling's in-flight work.** Stage `HEAD + rename` with `git hash-object -w` plus `git update-index --cacheinfo`: that commits the repoint alone and leaves their edits uncommitted in the working tree.
- **Do not reach for index-only staging when a plain `git add` is honest.** It exists for a rename that must reach files somebody else is holding, and it makes the index differ from the working tree on purpose.

## Files

- `scripts/dev/line_refs.py` (new), `c_line_number_ref` in `.claude/hooks/pretool-writeedit.py`
- `scripts/dev/rules_condensed.py` (two artifacts, not three)
- `ai/rules/` merged to 26 files: see [[1228-rule-format-condensed-eager-load]] and [[1287-rule-routing-severity-and-wip-cap]] for the routing design this preserves
