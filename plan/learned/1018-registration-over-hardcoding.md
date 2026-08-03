# 1018 -- Registration over hardcoding (enforced in every spec)

## Context

A spec-authored feature (the traffic monitor) was implemented by another agent that
hardcoded a per-feature field + factory + state + dispatch into the core CLI
`Model` (mirroring how dashboard/traceroute/ping already do it), instead of
registering and letting the core discover it. That violates the small-core /
registration pattern. The principle existed in `plugins.md` for the
daemon command/schema tree ("Unowned verb roots"), but it was never stated for the
CLI client and was not a per-spec check, so it slipped through review. Goal: make
"registration over hardcoding" land in EVERY spec we author and be mechanically
checkable.

## Decisions

- Extended `ai/rules/plugins.md` with a "Registration over hardcoding
  (the CLI client too)" section, over creating a new rule file -- the principle
  already lived there for commands/schema; generalize rather than proliferate rules.
- Propagated via `plan/TEMPLATE.md` (Architectural Verification bullet + Critical
  Review Checklist row), over relying on memory -- every spec is authored from the
  template, so it inherits the check.
- Enforced in `.claude/hooks/validate-spec.sh` as a WARNING, not a hard ERROR,
  because the hook re-runs on any spec Write/Edit and a hard error would fail all
  ~60 pre-existing specs the moment they are touched. Matched the repo's own
  grandfathering convention (the Risks & Assumptions and Acceptance Criteria checks
  are warnings for the same reason).
- One canonical phrase, "Registration over hardcoding", is the shared keyword across
  the template rows, the hook grep, and per-spec review rows.

## Consequences

- New specs carry the check by default; `/ze-review` reads the Critical Review
  Checklist row as blocking, so it is enforced at review time for new work.
- Existing specs are grandfathered: the hook only warns, surfacing the gap on edit
  without breaking the build.
- Reviewers get a mechanical test: "a new feature must not require editing a
  switch/case/field-list/factory in a core or shared package -- it registers and is
  discovered."
- Reusable shape for adding ANY future spec-gate: template (propagate) + warning hook
  (grandfather) + rule (rationale) + a single canonical keyword.

## Gotchas

- `validate-spec.sh` validates one spec at a time (reads the tool JSON from stdin on
  Write/Edit), not the whole tree -- so a hard ERROR there blocks editing existing
  specs; WARNING is the right severity.
- `ai/rules/INDEX.md` is generated from each rule's opening paragraph
  (`scripts/dev/rules_index.py`), and `CLAUDE.md`/`AGENTS.md` reference rules by path
  -- editing a rule's BODY (not its title/summary) needs no regen, and extending an
  existing rule (vs adding a file) avoids touching generated files entirely.
- The CLI-client hardcoding predates this (dashboard/traceroute/ping); the rule
  documents the `{prefix, factory, renderer}` registry fix, but migrating those
  existing views is separate scope.

## Files

- `plan/TEMPLATE.md` -- Architectural Verification + Critical Review Checklist rows
- `ai/rules/plugins.md` -- "Registration over hardcoding (the CLI client too)"
- `.claude/hooks/validate-spec.sh` -- warning check for the canonical phrase
