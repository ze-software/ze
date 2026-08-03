# 1006: Feature Surface Gate (generalizing the BGP Family Gate)

Spec: post-mortem of feature-then-fix commit pairs (geodns, firewall-irr, plugin
doctor registration, PATHS-LIMIT), prompted by a review of what we forget when
adding features.

## Context

Git history shows features routinely ship in 2-3 commits, not 1. The forgotten
piece is almost always a known integration surface, not an edge case:
composition-root regen + golden snapshots (geodns: `d84b2f433` -> `cdf5340e2`),
server-side RPC forwarder for a plugin's CLI commands (firewall-irr: `d9afefd2d`
-> `ef9e681fa`), the `ze doctor` check for a new runtime dependency
(`886-doctor-plugin-registration`), and stale interop fixtures after a config
restructure (PATHS-LIMIT: `90d860a40` -> `6f2e9c7e4`).

Lesson 905 already solved this for BGP families with a BLOCKING gate that forces
reading `ai/patterns/bgp-family.md` and filling a checklist. It worked: BGP
features dropped to 1 commit. But the forcing function was never generalized.
For plugins, components, CLI commands, and config options, the spec skill left
surface enumeration to memory, even though `plan/TEMPLATE.md` already carried a
generic Integration Checklist (with a doctor-check row) and a 17-row
Documentation Update Checklist, and `ai/INDEX.md` already mapped feature type to
pattern doc. Those existed but the skill never told the author to fill them.

Two surfaces were each verified in only one of the three workflows: the doctor
check (pushed in /ze-implement, checked nowhere in /ze-review) and composition
root (checked in /ze-review, not in /ze-spec or /ze-implement). The doctor check
was the only surface verified nowhere at review time. Separately,
`ai/rules/repo-maintenance.md` (the canonical "every surface a change must
touch" rule) was referenced by none of the three skills.

## Decisions

**Generalize the proven gate rather than invent a new artifact.** The per-type
pattern docs (`plugin.md`, `cli-command.md`, `config-option.md`, `bgp-family.md`)
and the template checklists already enumerate the surfaces. The gap was wiring,
not content. The new Feature Surface Gate in /ze-spec Step 2 (RESEARCH) reuses
the existing `ai/INDEX.md` feature-type-to-pattern map and the existing template
checklists. No duplicated list to drift. The BGP Family Gate stays as the
strongest protocol sub-case.

**Force the template checklists at WRITE and Pre-Spec Verification.** The
Integration Checklist and Documentation Update Checklist were added to the Step 4
fill-list and the Pre-Spec Verification gate, so they can no longer survive as
empty template placeholders. The doctor row must be answered when the feature
adds any runtime dependency.

**Plug the doctor surface at review.** /ze-review Step 1 gained a
runtime-dependency row: a new dependency with no registered `ze doctor` check and
diagnostic code is a BLOCKER. This is the surface that was gated nowhere else.

**Close the discovery hole.** Both skills now reference
`ai/rules/repo-maintenance.md` (spec via the gate's Mechanical Checklist step,
review via a new doc-drift row for `ai/INDEX.md` / `ai/LEARNED-INDEX.md` /
`repo-maintenance.md`). The rule is no longer orphaned from the workflows.

## Consequences

- Every feature spec (not just BGP) must enumerate its surfaces before design:
  read the matching pattern doc, fill both template checklists, answer the
  discovery checklist.
- /ze-review now catches a missing doctor check and a missing discovery-index
  update.
- Surface enumeration moves to the spec, so the work happens in one pass instead
  of being discovered at verify/review/external-review time.

## Gotchas

- The generated `.claude/skills/*/SKILL.md` copies are gitignored; edit the
  canonical `ai/skills/*.md` and run `make ze-ai-sync` (then `make ze-ai-check`
  confirms no drift). See `ai/rules/repo-maintenance.md`.
- Composition-root drift is already auto-caught at verify time
  (`ze-plugin-imports-check` + golden snapshots) and at review; the residual cost
  is that it is found late. Adding `make generate` to /ze-implement verification
  would move it earlier (deferred, not done here).
- A still-ungated surface: nothing automatically asserts "new runtime dependency
  => doctor check exists." The review row is the human backstop; a heuristic in
  `scripts/dev/verify_wiring_docs.py` would make it automatic (deferred).

## Files

- `ai/skills/ze-spec.md` -- Feature Surface Gate added to Step 2; both checklists
  added to Step 4 fill-list and Pre-Spec Verification.
- `ai/skills/ze-review.md` -- doctor-check row (Step 1), discovery-index row
  (Step 3 doc-drift).
- Reuses existing `ai/INDEX.md` type-to-pattern map, `plan/TEMPLATE.md`
  checklists, `ai/rules/repo-maintenance.md`, `ai/rules/repo-maintenance.md`.
- Generalizes lesson 905 (`ai/patterns/bgp-family.md`).
