---
name: ze-disposition
description: Judge whether an old spec still belongs in today's main before anyone implements it, and record one verdict in the spec (CURRENT, REWORK, SUPERSEDED, OBSOLETE). Use when picking up a skeleton, design or ready spec that has not been touched in weeks, when its targets look moved or gone, or when asked to triage or prune stale specs.
---

# Spec Disposition

Judge whether a spec written against an older tree still belongs in today's
`main`, before anyone implements it. Answers: **"Should this spec still run, as
written?"**

A spec names the code, the pages and the design it was written against. By the
time someone picks it up, that code may have moved, another spec may have
solved the problem, or a rule in `ai/META-DECISIONS.md` may have changed the
ground under it. An agent that implements it unchecked builds a duplicate, or
follows a design `main` has left. This skill does the check once and records
the verdict in the spec.

It is triage. It changes no code, redesigns nothing, and never deletes a spec.

See also: `/ze-audit` (what of a live spec is already built), `/ze-spec`
(rework a design), `/ze-status` (the attention view that lists stale specs)

## Input

- A spec stem or path: judge that spec.
- `stale`: take the candidates from `./le spec status report`, the rows it
  flags first, then `design` and `ready` specs by oldest `Updated`.

Skip `in-progress` and `verification` specs unless the user names one: they
have an owner, and the owner judges them.

## Delegation

`ai/rules/planning.md`: the main thread supervises, it does not run this
phase itself.

- **If you are the main thread:** spawn one agent per spec with
  `subagent_type: ze-read`, hand it the spec path and this skill, and run at
  most five at once. You do not need to ask permission first
  (`ai/META-DECISIONS.md`, "Delegate to subagents"). The agents change no file:
  you verify each report and write the record yourself ("Record the verdict"
  below).
- **If you are that agent:** run "Judge the spec" below. Resolve symbols with
  the LSP tool or `gopls` (`ai/rules/context-economy.md`). You cannot ask the
  user, so put any question in your report.
- **Either way:** every claim names its evidence: a file plus the symbol that
  produces the behavior, a commit SHA, or a spec path (`ai/rules/evidence.md`).
  Report the verdict and the evidence that would overturn it, never the
  search. Under 40 lines.

## Judge the spec

1. **Read the spec in full:** Task, Required Reading, Files to Modify and
   Create, acceptance criteria, `Depends`, `Release reason`, and its folder,
   which says what the first release owes (`plan/README.md`). Find the tree it
   was written against: `git log -1 --format='%h %as' -- <spec path>` is the
   baseline.
2. **Read the pages first (`ai/rules/documentation.md`):** `ai/CODE-TO-DOCS.md`
   names the pages for the files the spec targets. A page that changed since
   the baseline is often the fastest sign that the design moved. Read
   `ai/doors/system.md` and the door of every subsystem the spec touches
   (`ai/doors/README.md`) in full: a rework that drops an outside contract a
   door names (third-party plugins in any language, the process protocol, the
   public `pkg/` API) is not a fit for `main`, and the smallest redesign the
   verdict proposes MUST keep every one of them.
   The write hook refuses recording the verdict in the spec until these doors
   are read (`writeSpecDoorsRead`).
3. **Answer four questions, each with evidence:**

   | Question | How to answer it |
   |----------|------------------|
   | Do its targets still exist? | Every file and symbol it names, at HEAD. A gone path: `git log --oneline --diff-filter=DR <baseline>..HEAD -- <path>` finds the move or the deletion |
   | Has the problem already been solved? | `git log --oneline <baseline>..HEAD -- <target files>`; other live specs under `plan/` naming the same targets or goal; closed specs in commit subjects. For a defect, read the producing function at HEAD and run the scoped test or reproduction that shows whether it still occurs |
   | Has the ground shifted? | `ai/META-DECISIONS.md` (release timing, RFC validation scope, YANG, ExaBGP), the subsystem's door in `ai/doors/README.md`, and each spec in `Depends`: closed or not, and whether it landed the shape this spec assumed |
   | Would it undo a decision? | `git log -S<symbol>` for a deliberate removal and its stated reason. Implementing it as written must not bring that back, or add a second path beside one that exists (`ai/rules/no-layering.md`) |

4. **Give exactly one verdict:**

   | Verdict | Meaning | Evidence it needs |
   |---------|---------|-------------------|
   | CURRENT | The goal holds and the design still fits `main` | Every target resolved at HEAD, nothing landed that does the work, no rule moved under it. List any stale references with their current location |
   | REWORK | The goal holds, the design does not fit `main` | What changed and where, and the smallest redesign that would fit |
   | SUPERSEDED | Something already landed does this work | The commit or spec that did it, and the producing function at HEAD that now delivers each acceptance criterion |
   | OBSOLETE | The goal itself no longer applies | The rule, decision or removal that ended it, quoted with its source |

Rules for the verdict:

- Age is not evidence, and neither is a missing file. A spec whose targets
  vanished can still have a live goal: that is REWORK.
- Partly done is REWORK, never SUPERSEDED. Name what landed and what remains.
- A spec under `plan/release/fixes/` is SUPERSEDED or OBSOLETE only when the
  defect no longer occurs at HEAD, shown at the producer. A skeleton status
  never makes a release defect optional (`plan/README.md`).
- An absent requirement of an RFC Ze implements is owed work, never OBSOLETE
  (`ai/rules/rfc-compliance.md`). It can be REWORK.
- When the evidence does not settle one verdict, report REWORK, name what you
  could not verify, and say what would settle it.

## Record the verdict

The main thread verifies each report against source before writing anything.
Then, in the spec:

1. Append a `## Disposition` section: a two-column table with `Date`, `Checked
   at` (the HEAD short SHA), `Verdict`, and one line per piece of evidence,
   followed by the agent's reasoning rewritten in prose. No code
   (`ai/rules/spec-no-code.md`). A later disposition replaces the section.
2. By verdict:
   - **CURRENT:** repoint each stale reference to its current location without
     changing what the spec asks for, and set `Updated` to today.
   - **REWORK:** set `Updated` to today, and set `Status` back to `design` if it
     was `ready`. The redesign runs through `/ze-spec` in its own session.
   - **SUPERSEDED, OBSOLETE:** set `Updated` to today and leave the spec in
     place. Removing a spec removes scope, and that is Thomas's decision
     (`ai/rules/principles.md`). Once he approves, the removal goes through
     `./le commit create` with `remove`, citations of the spec are repointed
     in the same commit (`ai/rules/planning.md`), and the commit tool states
     anything else it owes.
3. Commit the batch's spec edits as one docs-only commit through
   `./le commit create`. It owes no test run.

## Report

Open with the table of decisions Thomas owns: one row per SUPERSEDED or
OBSOLETE spec, with the verdict, the deciding evidence in one line, and what
closing it would drop. Then one line of counts per verdict, and the specs moved
back to `design`. Under 15 lines (`ai/META-DECISIONS.md`, "Reports and
messages").
