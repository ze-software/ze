---
name: ze-spec
description: Use when working in the Ze repo and the user asks for ze-spec or wants a new or resumed implementation spec. Work through scope, research, design, and write gates; capture actionable constraints from the codebase; and produce a spec that another session can implement without hidden context.
---

# Ze Spec

This skill manages the Ze spec-writing workflow.

## Workflow

1. Detect whether a selected spec already exists. Resume it if present; otherwise start a new one.
2. Scope gate:
   - find related active specs and learned summaries
   - suggest a filename
   - raise the strongest scope concern before asking for confirmation
3. Research gate:
   - read `ai/INDEX.md`, relevant architecture docs, and real source files
   - capture actionable `Decision` and `Constraint` notes
   - trace the data flow that the spec will touch
4. Design gate:
   - present at least two approaches
   - recommend one
   - define ACs, tests, file lists, and failure modes
5. Write gate:
   - update `plan/spec-<name>.md`
   - keep the spec implementation-ready and self-contained
   - update `tmp/session/selected-spec`

## Rules

- Ask one crisp question when a gate genuinely needs user confirmation; otherwise keep moving with repo evidence.
- Never rely on unstated context. If it matters, write it into the spec.
