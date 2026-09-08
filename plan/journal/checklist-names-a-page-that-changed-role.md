# A checklist step names a page that changed role

A page gets split (a dispatch page plus one or more detail pages), or a file
moves and a redirect stub takes its old name. The checklist step that told an
agent to check that page for drift was never updated, so it keeps naming the
old target. Nothing fails: the step still resolves to a real file, so an agent
runs it against the wrong page and reports the check done.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-07 | none (weekly checklist follow-up) | `website/AI.md` Phase 4 step 2 (the canonical checklist) and its copy in `ai/skills/ze-weekly-update.md` Phase 4 step 3 | `website/compare/comparison.md` stopped being the BGP mirror when the page split into a dispatch page (`comparison.md`) plus `website/compare/bgp.md` and `website/compare/nos.md`. Both checklist copies still named the dispatch page as the mirror to edit, and `website/AI.md` also told the editor to bump an "as of" date in the mirror's disclaimer, a field the current mirror format does not carry. Found while syncing `website/compare/bgp.md` against seven commits' worth of drift in `docs/comparison.md`, at the direction of a task that already named the correct target | fixed: both files now name `website/compare/bgp.md` as the mirror, and the stale "as of" date instruction is removed from `website/AI.md`. Left open: `ai/skills/ze-weekly-update.md` restates a checklist `website/AI.md` already owns, so the two can drift again independently |
