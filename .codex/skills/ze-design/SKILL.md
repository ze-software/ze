---
name: ze-design
description: Use when working in the Ze repo and the user asks for ze-design or wants a design stress test. Ground every recommendation in repo rules and existing code patterns, work one decision at a time, and keep a running decision log.
---

# Ze Design

This skill is for design work that must stay aligned with Ze's existing rules and architecture.

## Workflow

1. Read the relevant constraints from `CLAUDE.md`, `AGENT.md`, `ai/INDEX.md`, and applicable `ai/rules/*`.
2. Read the plan, spec, or design under discussion.
3. Explore the current code paths that already solve similar problems.
4. Separate what is already decided from what still needs a decision.
5. Resolve one decision at a time: state the question, name the applicable rules, recommend an answer grounded in the codebase, then explain the trade-offs.
6. Keep a concise decision log the user could reuse in a spec.

## Rules

- Prefer repo evidence over generic industry patterns.
- Ask only crisp, single-decision questions.
- Surface your strongest concern before closing the design discussion.
